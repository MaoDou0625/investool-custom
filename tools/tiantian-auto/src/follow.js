import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const toolRoot = path.resolve(__dirname, "..");
const repoRoot = path.resolve(toolRoot, "..", "..");

const defaults = {
  holdingPageUrl: "https://trade.1234567.com.cn/MyAssets/Default",
  loginUrl: "https://login.1234567.com.cn/?direct_url=http://trade.1234567.com.cn/MyAssets/Default",
  importUrl: "http://127.0.0.1:4869/fund/portfolio/tiantian/import/xlsx/json",
  profileDir: path.join(repoRoot, ".local", "tiantian-profile"),
  downloadDir: path.join(repoRoot, ".local", "tiantian-downloads"),
  browserChannel: "msedge",
  browserExecutablePath: "",
  autoLogin: true,
  headless: true,
  timeoutMs: 60000,
  save: true,
  downloadSelectors: [],
  downloadButtonText: ["\u5bfc\u51fa", "\u4e0b\u8f7d", "Excel", "xlsx"],
  loginDetectionText: ["\u767b\u5f55", "\u8bf7\u767b\u5f55", "\u4ea4\u6613\u8d26\u53f7", "\u626b\u7801\u767b\u5f55"],
  readyDetectionText: ["\u6211\u7684\u8d44\u4ea7", "\u6301\u4ed3", "\u8d44\u4ea7\u660e\u7ec6"]
};

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const config = loadConfig(args);
  const mode = args.setupLogin ? "setup_login" : "follow";

  fs.mkdirSync(config.profileDir, { recursive: true });
  fs.mkdirSync(config.downloadDir, { recursive: true });

  const context = await chromium.launchPersistentContext(config.profileDir, {
    acceptDownloads: true,
    channel: config.browserExecutablePath ? undefined : config.browserChannel,
    downloadsPath: config.downloadDir,
    executablePath: config.browserExecutablePath || undefined,
    headless: args.headed ? false : config.headless,
    locale: "zh-CN"
  });

  try {
    const page = context.pages()[0] || await context.newPage();
    page.setDefaultTimeout(config.timeoutMs);

    if (mode === "setup_login") {
      await setupLogin(page, config);
      printStatus({ status: "login_profile_saved", profileDir: config.profileDir });
      return;
    }

    const downloadedFile = await downloadHoldingXLSX(page, config);
    const importResult = await importHoldingXLSX(downloadedFile, config);
    printStatus({
      status: "imported",
      downloadedFile,
      importUrl: config.importUrl,
      import: importResult
    });
  } finally {
    await context.close();
  }
}

function parseArgs(argv) {
  const args = {
    configPath: process.env.TIANTIAN_AUTO_CONFIG || "",
    setupLogin: false,
    headed: false
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    switch (arg) {
      case "--config":
        args.configPath = argv[++i] || "";
        break;
      case "--setup-login":
        args.setupLogin = true;
        args.headed = true;
        break;
      case "--headed":
        args.headed = true;
        break;
      case "--preview":
        args.save = false;
        break;
      case "--save":
        args.save = true;
        break;
      case "--holding-url":
        args.holdingPageUrl = argv[++i] || "";
        break;
      case "--import-url":
        args.importUrl = argv[++i] || "";
        break;
      case "--timeout-ms":
        args.timeoutMs = Number(argv[++i] || 0);
        break;
      default:
        throw new Error(`unknown argument: ${arg}`);
    }
  }
  return args;
}

function loadConfig(args) {
  let config = { ...defaults };
  if (args.configPath) {
    const configPath = path.resolve(args.configPath);
    const raw = fs.readFileSync(configPath, "utf8");
    config = { ...config, ...JSON.parse(raw) };
  }
  if (args.holdingPageUrl) {
    config.holdingPageUrl = args.holdingPageUrl;
  }
  if (args.importUrl) {
    config.importUrl = args.importUrl;
  }
  config.username = process.env.TIANTIAN_USERNAME || config.username || "";
  config.password = process.env.TIANTIAN_PASSWORD || config.password || "";
  if (Number.isFinite(args.timeoutMs) && args.timeoutMs > 0) {
    config.timeoutMs = args.timeoutMs;
  }
  if (typeof args.save === "boolean") {
    config.save = args.save;
  }
  config.profileDir = path.resolve(config.profileDir);
  config.downloadDir = path.resolve(config.downloadDir);
  return config;
}

async function setupLogin(page, config) {
  await page.goto(config.loginUrl || config.holdingPageUrl, { waitUntil: "domcontentloaded" });
  process.stderr.write(
    [
      "Complete TianTian login in the opened browser window.",
      "If the page asks for QR/SMS verification, finish it there.",
      "Press Enter here after the account page is logged in."
    ].join("\n") + "\n"
  );
  await waitForEnter();
  await page.goto(config.holdingPageUrl, { waitUntil: "domcontentloaded" }).catch(() => undefined);
  await page.waitForLoadState("networkidle", { timeout: 10000 }).catch(() => undefined);
}

async function downloadHoldingXLSX(page, config) {
  await page.goto(config.holdingPageUrl, { waitUntil: "domcontentloaded" });
  await page.waitForLoadState("networkidle", { timeout: Math.min(config.timeoutMs, 15000) }).catch(() => undefined);

  const state = await classifyPage(page, config);
  if (state === "login_required") {
    const loggedIn = await tryAutoLogin(page, config);
    if (!loggedIn) {
      throw statusError("login_required", "TianTian login is required or the saved browser session expired.");
    }
    await page.goto(config.holdingPageUrl, { waitUntil: "domcontentloaded" });
    await page.waitForLoadState("networkidle", { timeout: Math.min(config.timeoutMs, 15000) }).catch(() => undefined);
  }

  const trigger = await findDownloadTrigger(page, config);
  if (!trigger) {
    throw statusError("download_trigger_not_found", "No export/download control was found on the configured holding page.");
  }

  const download = await Promise.all([
    page.waitForEvent("download", { timeout: config.timeoutMs }),
    trigger.click()
  ]).then(([event]) => event);

  const suggested = sanitizeFilename(download.suggestedFilename() || `tiantian-holding-${timestamp()}.xlsx`);
  if (!suggested.toLowerCase().endsWith(".xlsx")) {
    throw statusError("download_not_xlsx", `Downloaded file is not an xlsx: ${suggested}`);
  }
  const target = path.join(config.downloadDir, `${timestamp()}-${suggested}`);
  await download.saveAs(target);
  return target;
}

async function classifyPage(page, config) {
  const currentURL = page.url();
  if (/login\.1234567\.com\.cn/i.test(currentURL)) {
    return "login_required";
  }
  const body = await page.locator("body").innerText({ timeout: 5000 }).catch(() => "");
  const hasReadyText = config.readyDetectionText.some((term) => body.includes(term));
  const hasLoginText = config.loginDetectionText.some((term) => body.includes(term));
  if (!hasReadyText && hasLoginText) {
    return "login_required";
  }
  return "ready";
}

async function tryAutoLogin(page, config) {
  if (!config.autoLogin || !config.username || !config.password) {
    return false;
  }

  await page.goto(config.loginUrl || config.holdingPageUrl, { waitUntil: "domcontentloaded" });
  await page.waitForLoadState("networkidle", { timeout: Math.min(config.timeoutMs, 15000) }).catch(() => undefined);

  const hasAccountInput = await page.locator("#tbname").isVisible().catch(() => false);
  const hasPasswordInput = await page.locator("#tbpwd").isVisible().catch(() => false);
  if (!hasAccountInput || !hasPasswordInput) {
    return false;
  }

  await page.locator("#tbname").fill(config.username);
  await page.locator("#tbpwd").fill(config.password);
  await checkVisibleBox(page, "#protocolCheckbox");
  await checkVisibleBox(page, "#tbcook");

  const loginButton = page.locator("#btn_login").first();
  if (!await isUsable(loginButton)) {
    return false;
  }

  await Promise.all([
    page.waitForURL((url) => !/login\.1234567\.com\.cn/i.test(String(url)), { timeout: Math.min(config.timeoutMs, 20000) }).catch(() => undefined),
    loginButton.click()
  ]);
  await page.waitForLoadState("networkidle", { timeout: Math.min(config.timeoutMs, 15000) }).catch(() => undefined);

  const postLoginState = await classifyPage(page, config);
  if (postLoginState === "login_required") {
    const message = await page.locator("#errmsg").innerText({ timeout: 1000 }).catch(() => "");
    const err = statusError(
      "login_failed",
      message || "TianTian account/password login did not complete; manual verification may be required."
    );
    throw err;
  }
  return true;
}

async function checkVisibleBox(page, selector) {
  const box = page.locator(selector).first();
  if (!await isUsable(box)) {
    return;
  }
  if (!await box.isChecked().catch(() => false)) {
    await box.check({ force: true });
  }
}

async function findDownloadTrigger(page, config) {
  for (const selector of config.downloadSelectors || []) {
    const locator = page.locator(selector).first();
    if (await isUsable(locator)) {
      return locator;
    }
  }

  const terms = (config.downloadButtonText || []).map((term) => String(term).trim()).filter(Boolean);
  if (terms.length === 0) {
    return null;
  }
  const pattern = new RegExp(terms.map(escapeRegExp).join("|"), "i");
  const controls = page.locator("a,button,input[type='button'],input[type='submit']").filter({ hasText: pattern });
  if (await isUsable(controls.first())) {
    return controls.first();
  }

  const inputByValue = page.locator("input[type='button'],input[type='submit']");
  const count = await inputByValue.count().catch(() => 0);
  for (let i = 0; i < count; i += 1) {
    const candidate = inputByValue.nth(i);
    const value = await candidate.getAttribute("value").catch(() => "");
    if (value && pattern.test(value) && await isUsable(candidate)) {
      return candidate;
    }
  }
  return null;
}

async function isUsable(locator) {
  try {
    await locator.waitFor({ state: "visible", timeout: 1500 });
    return await locator.isEnabled();
  } catch {
    return false;
  }
}

async function importHoldingXLSX(filename, config) {
  const buffer = fs.readFileSync(filename);
  const data = new FormData();
  data.append("action", config.save ? "save" : "preview");
  data.append(
    "holding_xlsx",
    new Blob([buffer], { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" }),
    path.basename(filename)
  );

  const response = await fetch(config.importUrl, { method: "POST", body: data });
  const text = await response.text();
  let payload;
  try {
    payload = JSON.parse(text);
  } catch {
    payload = { raw: text };
  }
  if (!response.ok) {
    const err = statusError("import_failed", `Local import endpoint returned HTTP ${response.status}.`);
    err.payload = payload;
    throw err;
  }
  return payload;
}

function statusError(status, message) {
  const err = new Error(message);
  err.status = status;
  return err;
}

function printStatus(payload) {
  process.stdout.write(JSON.stringify(payload, null, 2) + "\n");
}

function waitForEnter() {
  return new Promise((resolve) => {
    process.stdin.resume();
    process.stdin.once("data", () => {
      process.stdin.pause();
      resolve();
    });
  });
}

function sanitizeFilename(value) {
  return value.replace(/[<>:"/\\|?*\x00-\x1f]/g, "_");
}

function timestamp() {
  return new Date().toISOString().replace(/[-:.TZ]/g, "").slice(0, 14);
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

main().catch((err) => {
  const payload = {
    status: err.status || "failed",
    error: err.message
  };
  if (err.payload) {
    payload.payload = err.payload;
  }
  printStatus(payload);
  if (payload.status === "login_required") {
    process.exit(2);
  }
  if (payload.status === "download_trigger_not_found") {
    process.exit(3);
  }
  if (payload.status === "import_failed") {
    process.exit(4);
  }
  if (payload.status === "login_failed") {
    process.exit(5);
  }
  if (payload.status === "download_not_xlsx") {
    process.exit(7);
  }
  process.exit(1);
});
