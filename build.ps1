$v = git describe --tags --always 2>$null
if (!$v) { $v = "dev" }

$dir = "cmd\\build"
mkdir $dir -Force | Out-Null
rm "$dir\*" -Force -ErrorAction SilentlyContinue

Write-Host "Building..." -ForegroundColor Yellow

$platforms = @(
    "windows/amd64/.exe",
    "windows/386/.exe",
    "linux/amd64/",
    "linux/386/",
    "linux/arm64/",
    "darwin/amd64/",
    "darwin/arm64/"
)

$ok = 0
$fail = 0

foreach ($p in $platforms) {
    $parts = $p -split "/"
    $os = $parts[0]
    $arch = $parts[1]
    $ext = $parts[2]
    
    $out = "$dir/goevtxcarver-$os-$arch$ext"
    Write-Host "  $os/$arch... " -NoNewline
    
    $env:GOOS = $os
    $env:GOARCH = $arch
    $env:CGO_ENABLED = "0"
    
    go build -ldflags="-s -w" -o $out 'cmd\\evtx\\main.go' 2>$null
    
    if ($LASTEXITCODE -eq 0) {
        $size = [math]::Round((Get-Item $out).Length / 1MB, 2)
        Write-Host '✓ ($size MB)' -ForegroundColor Green
        $ok++
    } else {
        Write-Host '✗' -ForegroundColor Red
        $fail++
    }
}

Write-Host "`nDone: $ok ok, $fail failed" -ForegroundColor Cyan