param(
    [string]$Title = "zot-screenshot",
    [string]$Commands,
    [string]$OutputDir = "D:\C\Documents\Program\Go\zotero_cli\docs\screenshots",
    [int]$DelayMs = 2000
)

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

$cmdContent = $Commands + "`npause"
$tmpScript = Join-Path $env:TEMP ("zot-capture-" + [guid]::NewGuid().ToString("N") + ".bat")
$cmdContent | Set-Content -Path $tmpScript -Encoding ASCII

$proc = Start-Process cmd.exe -ArgumentList "/k", ("title " + $Title + " && `"" + $tmpScript + "`"") -PassThru

Start-Sleep -Milliseconds $DelayMs

$hwnd = [IntPtr]::Zero
for ($i = 0; $i -lt 15; $i++) {
    Start-Sleep -Milliseconds 400
    $procs = [System.Diagnostics.Process]::GetProcessesByName("cmd")
    foreach ($p in $procs) {
        if ($p.MainWindowTitle -like ("*" + $Title + "*")) {
            $hwnd = $p.MainWindowHandle
            break
        }
    }
    if ($hwnd -ne [IntPtr]::Zero) { break }
}

if ($hwnd -eq [IntPtr]::Zero) {
    Write-Host ("ERROR: Window not found: " + $Title) -ForegroundColor Red
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    Remove-Item $tmpScript -Force -ErrorAction SilentlyContinue
    exit 1
}

Start-Sleep -Milliseconds 1000

$sig = @"
[DllImport("user32.dll")]
public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
[StructLayout(LayoutKind.Sequential)]
public struct RECT { public int Left; public int Top; public int Right; public int Bottom; }
"@
$null = Add-Type -MemberDefinition $sig -Name "Win32Rect" -Namespace Win32Func -PassThru
$rect = New-Object Win32Func.Win32Func+RECT
[Win32Func.Win32Func]::GetWindowRect($hwnd, [ref]$rect) | Out-Null

$windowW = $rect.Right - $rect.Left
$windowH = $rect.Bottom - $rect.Top

$bmp = New-Object System.Drawing.Bitmap($windowW, $windowH)
$graphics = [System.Drawing.Graphics]::FromImage($bmp)
$graphics.CopyFromScreen($rect.Left, $rect.Top, 0, 0, (New-Object System.Drawing.Size($windowW, $windowH)))

$outFile = Join-Path $OutputDir ($Title + ".png")
$bmp.Save($outFile, [System.Drawing.Imaging.ImageFormat]::Png)

$graphics.Dispose()
$bmp.Dispose()

Write-Host ("OK: " + $outFile) -ForegroundColor Green

Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
Remove-Item $tmpScript -Force -ErrorAction SilentlyContinue
