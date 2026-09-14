param(
    [Parameter(Mandatory = $true)]
    [string]$ExecutablePath
)

$ErrorActionPreference = "Stop"
$resolvedExecutable = (Resolve-Path $ExecutablePath).Path
$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("obs-network-monitor-smoke-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $testRoot | Out-Null

function Stop-MonitorProcess {
    param([System.Diagnostics.Process]$Process)

    if ($null -ne $Process -and -not $Process.HasExited) {
        Stop-Process -Id $Process.Id -Force
        $Process.WaitForExit()
    }
}

function Receive-MonitorSnapshot {
    $socket = [System.Net.WebSockets.ClientWebSocket]::new()
    $cancellation = [System.Threading.CancellationTokenSource]::new([TimeSpan]::FromSeconds(10))
    $stream = [System.IO.MemoryStream]::new()
    try {
        $uri = [Uri]::new("ws://127.0.0.1:8080/ws")
        $socket.ConnectAsync($uri, $cancellation.Token).GetAwaiter().GetResult()
        $buffer = New-Object byte[] 65536
        do {
            $segment = [ArraySegment[byte]]::new($buffer)
            $received = $socket.ReceiveAsync($segment, $cancellation.Token).GetAwaiter().GetResult()
            if ($received.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Close) {
                throw "WebSocket closed before a snapshot was received."
            }
            $stream.Write($buffer, 0, $received.Count)
        } while (-not $received.EndOfMessage)

        $json = [System.Text.Encoding]::UTF8.GetString($stream.ToArray())
        return $json | ConvertFrom-Json
    }
    finally {
        $stream.Dispose()
        $socket.Dispose()
        $cancellation.Dispose()
    }
}

function Assert-MonitorSnapshot {
    param(
        [object]$Snapshot,
        [string[]]$ExpectedTargets
    )

    if ($null -eq $Snapshot.nic) {
        throw "Snapshot does not contain NIC information."
    }
    if ($null -eq $Snapshot.statistics) {
        throw "Snapshot does not contain statistics."
    }
    $statisticProperties = $Snapshot.statistics.PSObject.Properties.Name
    foreach ($property in @("averageLatencyMs", "minimumLatencyMs", "maximumLatencyMs", "consecutiveFailures")) {
        if ($statisticProperties -notcontains $property) {
            throw "Snapshot statistics do not contain $property."
        }
    }
    if (@("icmp", "http") -notcontains $Snapshot.statistics.method) {
        throw "Unexpected probe method: $($Snapshot.statistics.method)"
    }
    if ($ExpectedTargets -notcontains $Snapshot.statistics.target) {
        throw "Unexpected probe target: $($Snapshot.statistics.target)"
    }
    if (@("packetLoss", "requestFailure") -notcontains $Snapshot.statistics.failureMetric) {
        throw "Unexpected failure metric: $($Snapshot.statistics.failureMetric)"
    }
    if ($Snapshot.history.Count -gt 60) {
        throw "History contains more than 60 samples."
    }
    if ($null -eq $Snapshot.traffic) {
        throw "Snapshot does not contain current NIC traffic."
    }
    $trafficProperties = $Snapshot.traffic.PSObject.Properties.Name
    if ($trafficProperties -notcontains "transmitBps" -or $trafficProperties -notcontains "receiveBps") {
        throw "Snapshot traffic does not contain transmitBps and receiveBps."
    }
    if ($null -eq $Snapshot.trafficHistory -or $Snapshot.trafficHistory.Count -gt 60) {
        throw "Traffic history is missing or contains more than 60 samples."
    }
    if ($null -eq $Snapshot.generatedAt) {
        throw "Snapshot does not contain generatedAt."
    }
}

function Test-RunningMonitor {
    param(
        [string]$Name,
        [AllowNull()][string]$ConfigContent,
        [string[]]$ExpectedTargets
    )

    $scenarioDirectory = Join-Path $testRoot $Name
    New-Item -ItemType Directory -Path $scenarioDirectory | Out-Null
    $scenarioExecutable = Join-Path $scenarioDirectory "obs-network-monitor.exe"
    Copy-Item $resolvedExecutable $scenarioExecutable
    if (-not [string]::IsNullOrEmpty($ConfigContent)) {
        Set-Content -Path (Join-Path $scenarioDirectory "config.json") -Value $ConfigContent -Encoding utf8
    }

    $standardOutput = Join-Path $scenarioDirectory "stdout.log"
    $standardError = Join-Path $scenarioDirectory "stderr.log"
    $process = Start-Process -FilePath $scenarioExecutable `
        -WorkingDirectory $scenarioDirectory `
        -RedirectStandardOutput $standardOutput `
        -RedirectStandardError $standardError `
        -PassThru
    try {
        $response = $null
        for ($attempt = 0; $attempt -lt 40; $attempt++) {
            if ($process.HasExited) {
                $errorLog = Get-Content $standardError -Raw
                throw "Monitor exited before startup in scenario '$Name': $errorLog"
            }
            try {
                $response = Invoke-WebRequest -Uri "http://127.0.0.1:8080/" -TimeoutSec 2 -UseBasicParsing
                if ($response.StatusCode -eq 200) {
                    break
                }
            }
            catch {
                Start-Sleep -Milliseconds 250
            }
        }

        if ($null -eq $response -or $response.StatusCode -ne 200) {
            throw "Monitor HTTP endpoint did not become ready in scenario '$Name'."
        }
        if ($response.Content -notmatch "service-status" -or $response.Content -match "chart-canvas") {
            throw "Monitor status page was not returned in scenario '$Name'."
        }

        $displayPages = @(
            @{ Path = "latency"; Present = "latency-chart-canvas"; Absent = "traffic-chart-canvas" },
            @{ Path = "traffic"; Present = "traffic-chart-canvas"; Absent = "latency-chart-canvas" }
        )
        foreach ($displayPage in $displayPages) {
            $displayResponse = Invoke-WebRequest `
                -Uri "http://127.0.0.1:8080/$($displayPage.Path)?parts=graph" `
                -TimeoutSec 2 `
                -UseBasicParsing
            if ($displayResponse.StatusCode -ne 200 `
                -or $displayResponse.Content -notmatch $displayPage.Present `
                -or $displayResponse.Content -match $displayPage.Absent) {
                throw "Display endpoint '/$($displayPage.Path)' returned unexpected content in scenario '$Name'."
            }
        }

        $firstSnapshot = Receive-MonitorSnapshot
        $secondSnapshot = Receive-MonitorSnapshot
        Assert-MonitorSnapshot $firstSnapshot $ExpectedTargets
        Assert-MonitorSnapshot $secondSnapshot $ExpectedTargets
    }
    finally {
        Stop-MonitorProcess $process
    }
}

function Test-InvalidConfiguration {
    $scenarioDirectory = Join-Path $testRoot "invalid-config"
    New-Item -ItemType Directory -Path $scenarioDirectory | Out-Null
    $scenarioExecutable = Join-Path $scenarioDirectory "obs-network-monitor.exe"
    Copy-Item $resolvedExecutable $scenarioExecutable
    Set-Content -Path (Join-Path $scenarioDirectory "config.json") `
        -Value '{"unknownField":true}' `
        -Encoding utf8

    $standardOutput = Join-Path $scenarioDirectory "stdout.log"
    $standardError = Join-Path $scenarioDirectory "stderr.log"
    $process = Start-Process -FilePath $scenarioExecutable `
        -WorkingDirectory $scenarioDirectory `
        -RedirectStandardOutput $standardOutput `
        -RedirectStandardError $standardError `
        -PassThru
    try {
        if (-not $process.WaitForExit(10000)) {
            throw "Monitor did not reject an invalid configuration within 10 seconds."
        }
        if ($process.ExitCode -eq 0) {
            throw "Monitor accepted an invalid configuration."
        }
        $errorLog = Get-Content $standardError -Raw
        if ($errorLog -notmatch "load config") {
            throw "Invalid configuration error was not logged: $errorLog"
        }
    }
    finally {
        Stop-MonitorProcess $process
    }
}

try {
    Test-RunningMonitor `
        -Name "default-config" `
        -ConfigContent $null `
        -ExpectedTargets @("8.8.8.8", "https://www.google.com/generate_204")

    $validConfig = '{"icmpTarget":"1.1.1.1","httpTarget":"https://example.com/"}'
    Test-RunningMonitor `
        -Name "valid-config" `
        -ConfigContent $validConfig `
        -ExpectedTargets @("1.1.1.1", "https://example.com/")

    Test-InvalidConfiguration
    Write-Host "Windows integration smoke tests passed."
}
finally {
    if (Test-Path $testRoot) {
        Remove-Item $testRoot -Recurse -Force
    }
}
