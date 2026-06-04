param(
    [ValidateSet("Set", "Get", "Delete")]
    [string]$Action = "Get",
    [string]$Target = "investool-tiantian",
    [switch]$IncludeSecret
)

$ErrorActionPreference = "Stop"

if (-not ([System.Management.Automation.PSTypeName]'InvestoolCredential.Native').Type) {
    Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;

namespace InvestoolCredential {
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    public struct Credential {
        public UInt32 Flags;
        public UInt32 Type;
        public string TargetName;
        public string Comment;
        public System.Runtime.InteropServices.ComTypes.FILETIME LastWritten;
        public UInt32 CredentialBlobSize;
        public IntPtr CredentialBlob;
        public UInt32 Persist;
        public UInt32 AttributeCount;
        public IntPtr Attributes;
        public string TargetAlias;
        public string UserName;
    }

    public static class Native {
        [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        public static extern bool CredWrite(ref Credential userCredential, UInt32 flags);

        [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        public static extern bool CredRead(string target, UInt32 type, UInt32 reservedFlag, out IntPtr credentialPtr);

        [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        public static extern bool CredDelete(string target, UInt32 type, UInt32 flags);

        [DllImport("advapi32.dll")]
        public static extern void CredFree(IntPtr cred);
    }
}
"@
}

$credentialTypeGeneric = 1
$persistLocalMachine = 2

function Throw-LastWin32Error([string]$Message) {
    $code = [Runtime.InteropServices.Marshal]::GetLastWin32Error()
    throw "$Message Win32Error=$code"
}

function Set-InvestoolCredential {
    param([string]$TargetName)

    $prompt = "TianTian fund trading account for $TargetName"
    $credential = Get-Credential -Message $prompt
    $network = $credential.GetNetworkCredential()
    $passwordBytes = [Text.Encoding]::Unicode.GetBytes($network.Password)
    $blob = [Runtime.InteropServices.Marshal]::AllocHGlobal($passwordBytes.Length)
    try {
        [Runtime.InteropServices.Marshal]::Copy($passwordBytes, 0, $blob, $passwordBytes.Length)
        $nativeCredential = New-Object InvestoolCredential.Credential
        $nativeCredential.Type = $credentialTypeGeneric
        $nativeCredential.TargetName = $TargetName
        $nativeCredential.UserName = $network.UserName
        $nativeCredential.CredentialBlob = $blob
        $nativeCredential.CredentialBlobSize = $passwordBytes.Length
        $nativeCredential.Persist = $persistLocalMachine

        if (-not [InvestoolCredential.Native]::CredWrite([ref]$nativeCredential, 0)) {
            Throw-LastWin32Error "Failed to write Windows credential."
        }
    } finally {
        [Runtime.InteropServices.Marshal]::FreeHGlobal($blob)
    }

    [pscustomobject]@{
        Target = $TargetName
        UserName = $network.UserName
        Saved = $true
    } | ConvertTo-Json -Compress
}

function Get-InvestoolCredential {
    param(
        [string]$TargetName,
        [switch]$IncludeSecret
    )

    $credentialPtr = [IntPtr]::Zero
    if (-not [InvestoolCredential.Native]::CredRead($TargetName, $credentialTypeGeneric, 0, [ref]$credentialPtr)) {
        exit 2
    }

    try {
        $nativeCredential = [Runtime.InteropServices.Marshal]::PtrToStructure(
            $credentialPtr,
            [type][InvestoolCredential.Credential]
        )
        $payload = [ordered]@{
            Target = $TargetName
            UserName = $nativeCredential.UserName
            HasSecret = $nativeCredential.CredentialBlobSize -gt 0
        }
        if ($IncludeSecret -and $nativeCredential.CredentialBlobSize -gt 0) {
            $payload.Password = [Runtime.InteropServices.Marshal]::PtrToStringUni(
                $nativeCredential.CredentialBlob,
                [int]($nativeCredential.CredentialBlobSize / 2)
            )
        }

        [pscustomobject]$payload | ConvertTo-Json -Compress
    } finally {
        [InvestoolCredential.Native]::CredFree($credentialPtr)
    }
}

function Remove-InvestoolCredential {
    param([string]$TargetName)

    if (-not [InvestoolCredential.Native]::CredDelete($TargetName, $credentialTypeGeneric, 0)) {
        $code = [Runtime.InteropServices.Marshal]::GetLastWin32Error()
        if ($code -ne 1168) {
            throw "Failed to delete Windows credential. Win32Error=$code"
        }
    }

    [pscustomobject]@{
        Target = $TargetName
        Deleted = $true
    } | ConvertTo-Json -Compress
}

switch ($Action) {
    "Set" { Set-InvestoolCredential -TargetName $Target }
    "Get" { Get-InvestoolCredential -TargetName $Target -IncludeSecret:$IncludeSecret }
    "Delete" { Remove-InvestoolCredential -TargetName $Target }
}
