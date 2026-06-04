# TianTian Auto Follow

This tool automates the existing TianTian fund portfolio import path:

1. Reuse a persistent browser profile for TianTian login state.
2. Open the configured holding page.
3. Download the holding xlsx through configured selectors or common export labels.
4. Upload the xlsx to the local InvesTool JSON import endpoint.
5. Print a compact JSON status for Codex or scheduled jobs.

The script does not store account passwords in this repository. Save the TianTian credential in Windows Credential Manager, then later runs read it into temporary process environment variables and auto-fill the login page.

```powershell
scripts\tiantian_auto_follow.ps1 -SetCredential
scripts\tiantian_auto_follow.ps1 -SetupLogin
scripts\tiantian_auto_follow.ps1
```

`-SetupLogin` is still useful when TianTian asks for QR/SMS/captcha verification. After that, the persistent browser profile can be reused by daily runs.

If TianTian changes its page or the export button is not found, copy `tools\tiantian-auto\tiantian.auto.example.json.example` to a local ignored config file and add the exact export selector:

```powershell
Copy-Item tools\tiantian-auto\tiantian.auto.example.json.example .local\tiantian.auto.json
scripts\tiantian_auto_follow.ps1 -ConfigPath .local\tiantian.auto.json
```

The default local import URL is:

```text
http://127.0.0.1:4869/fund/portfolio/tiantian/import/xlsx/json
```
