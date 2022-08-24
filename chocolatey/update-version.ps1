$url = "https://github.com/CircleCI-Public/circleci-cli/releases/latest" 

$request = [System.Net.WebRequest]::Create($url)
$request.AllowAutoRedirect=$false
$response=$request.GetResponse()
$versionURL = $response.GetResponseHeader("Location")

$curVersion = ($versionURL | Split-Path -Leaf).Substring(1)

$tmpFile = ".\out.txt"
$checksumURL = "https://github.com/CircleCI-Public/circleci-cli/releases/download/v$curVersion/circleci-cli_$($curVersion)_checksums.txt"
Write-Verbose "getting checksum from $checksumURL to $tmpFile"

(New-Object System.Net.WebClient).DownloadString($checksumURL) >> $tmpFile

### replace hash
$hash = (Get-Content $tmpFile | where {$_ -like "*windows_amd64.zip"}).split(" ")[0]
$installerPath = ".\circleci-cli\tools\chocolateyinstall.ps1"
(Get-Content $installerPath).Replace('$HASH',$hash) | Out-File $installerPath -Force

$downloadURL = "https://github.com/CircleCI-Public/circleci-cli/releases/download/v$curVersion/circleci-cli_$($curVersion)_windows_amd64.zip"
(Get-Content $installerPath).Replace('$DOWNLOAD_URL',$downloadURL) | Out-File $installerPath -Force

$nuspecPath = "./circleci-cli/circleci-cli.nuspec"
(Get-Content $nuspecPath).Replace('$VER',$curVersion) | Out-File $nuspecPath -Force
