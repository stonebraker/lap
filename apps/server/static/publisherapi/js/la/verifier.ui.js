// verifier.ui.js — UI adapter (ES module)
import {
    nowSecs,
    fetchRA as fetchRAImpl,
    verifyFragment,
} from "./verifier.core.js";

// Friendly time formatter for hints
function fmtTime(ts) {
    try {
        return new Date(ts * 1000).toLocaleTimeString([], {
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
        });
    } catch {
        return String(ts);
    }
}

// Minimal renderer with optional de-dupe to avoid flashing
function setVerifyResult(resEl, ok, msg, opts = {}) {
    const text = String(msg || "");
    if (opts.onlyUpdateOnChange && resEl._lapLastMsg === text) return;
    resEl._lapLastMsg = text;
    resEl.textContent = text;
    resEl.style.color = ok ? "rgb(34,197,94)" : "crimson";
    resEl.title = text;
}

// ---- contract checks & parsing (DOM-aware) ----
function checkArticleContract(articleEl) {
    const spec = articleEl.dataset.lapSpec || "";
    const profile = articleEl.dataset.lapProfile || "";
    const attFmt = articleEl.dataset.lapAttestationFormat || "div";
    const bytesFmt = articleEl.dataset.lapBytesFormat || "link-data";
    if (!spec) return "missing data-lap-spec";
    if (profile !== "fragment") return 'data-lap-profile must be "fragment"';
    if (attFmt !== "div") return `unsupported attestation format: ${attFmt}`;
    if (bytesFmt !== "link-data")
        return `unsupported bytes format: ${bytesFmt}`;
    return null;
}

function parseStapled(articleEl) {
    const attSel = articleEl.dataset.lapAttestation || "";
    const attEl =
        (attSel && articleEl.querySelector(attSel)) ||
        articleEl.querySelector(".lap-attestation");
    const payEl = attEl && (attEl.querySelector(".lap-payload") || attEl);
    if (
        attEl &&
        payEl &&
        attEl.dataset.lapSig &&
        attEl.dataset.lapResourceKey
    ) {
        const p = {
            url: payEl.dataset.lapUrl,
            attestation_url: payEl.dataset.lapAttestationUrl,
            hash: payEl.dataset.lapHash,
            etag: payEl.dataset.lapEtag,
            iat: Number(payEl.dataset.lapIat),
            exp: Number(payEl.dataset.lapExp),
            kid: payEl.dataset.lapKid,
        };
        return {
            payload: p,
            resource_key: attEl.dataset.lapResourceKey,
            sig: attEl.dataset.lapSig,
        };
    }
    throw new Error("missing stapled attestation");
}

async function verifyCanonicalBytes(articleEl, expectedHashTag) {
    const sel = articleEl.dataset.lapBytes;
    if (!sel) return "missing data-lap-bytes selector";
    const link = articleEl.querySelector(sel);
    if (!link) return "data-lap-bytes selector did not resolve";
    const href = link.getAttribute("href") || "";
    if (!href.startsWith("data:")) return "bytes href is not a data: URL";
    const i = href.indexOf(",");
    if (i < 0) return "malformed data: URL";
    const meta = href.slice(5, i);
    const data = href.slice(i + 1);
    if (!/;base64(?:;|$)/i.test(meta)) return "data: URL must be base64";
    const bin = atob(data);
    const bytes = new Uint8Array(bin.length);
    for (let j = 0; j < bin.length; j++) bytes[j] = bin.charCodeAt(j);
    const d = await crypto.subtle.digest("SHA-256", bytes);
    const hex = [...new Uint8Array(d)]
        .map((b) => b.toString(16).padStart(2, "0"))
        .join("");
    const got = `sha256:${hex}`;
    if (got !== expectedHashTag)
        return "bytes hash mismatch (data URL vs payload.hash)";
    const declared = link.getAttribute("data-hash");
    if (declared && declared !== expectedHashTag)
        return "bytes data-hash mismatch";
    return null;
}

// ---- UI elements ----
function ensureControls(article) {
    try {
        console.debug(
            "[LAP] ensureControls for",
            article.dataset.lapUrl || article.id
        );
    } catch {}
    let bar = article.querySelector(".lap-verify-bar");
    if (!bar) {
        bar = document.createElement("div");
        bar.className = "lap-verify-bar";
        bar.style.display = "flex";
        bar.style.alignItems = "center";
        bar.style.gap = "0.5rem";
        bar.style.marginTop = "0.5rem";
        bar.style.fontSize = "0.875rem";
        const previewSel = article.dataset.lapPreview;
        const preview = previewSel ? article.querySelector(previewSel) : null;
        (preview?.parentNode || article).insertBefore(
            bar,
            preview?.nextSibling || article.firstChild
        );
    }
    let res = bar.querySelector(".lap-verify-result");
    if (!res) {
        res = document.createElement("span");
        res.className = "lap-verify-result";
        res.setAttribute("aria-live", "polite");
        res.style.minWidth = "1.25rem";
        bar.appendChild(res);
    }
    let verifyBtn = bar.querySelector(".lap-verify-btn");
    if (!verifyBtn) {
        verifyBtn = document.createElement("button");
        verifyBtn.type = "button";
        verifyBtn.className = "lap-verify-btn";
        verifyBtn.textContent = "Verify";
        Object.assign(verifyBtn.style, {
            padding: "0.25rem 0.5rem",
            border: "1px solid rgba(148,163,184,0.4)",
            borderRadius: "0.375rem",
            background: "transparent",
            color: "inherit",
            cursor: "pointer",
        });
        // Place button before the result span
        if (res && res.parentNode === bar) {
            bar.insertBefore(verifyBtn, res);
        } else {
            bar.appendChild(verifyBtn);
        }
    }
    let refreshBtn = bar.querySelector(".lap-refresh-btn");
    if (!refreshBtn) {
        refreshBtn = document.createElement("button");
        refreshBtn.type = "button";
        refreshBtn.className = "lap-refresh-btn";
        refreshBtn.title = "Refresh";
        Object.assign(refreshBtn.style, {
            padding: "0.25rem 0.5rem",
            border: "1px solid rgba(148,163,184,0.4)",
            borderRadius: "0.375rem",
            background: "transparent",
            color: "inherit",
            cursor: "pointer",
            display: "none",
        });
        refreshBtn.innerHTML =
            '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"></polyline><polyline points="1 20 1 14 7 14"></polyline><path d="M3.51 9a9 9 0 0 1 14.86-3.36L23 10"></path><path d="M20.49 15a9 9 0 0 1-14.86 3.36L1 14"></path></svg>';
        // Place refresh button immediately to the left of Verify
        if (verifyBtn && verifyBtn.parentNode === bar) {
            bar.insertBefore(refreshBtn, verifyBtn);
        } else {
            bar.appendChild(refreshBtn);
        }
    }
    let countdown = bar.querySelector(".lap-verify-countdown");
    if (!countdown) {
        countdown = document.createElement("span");
        countdown.className = "lap-verify-countdown";
        countdown.style.marginLeft = "0.5rem";
        countdown.style.opacity = "0.8";
        bar.appendChild(countdown);
    }
    // Wire click to re-run verification
    if (verifyBtn) {
        verifyBtn.onclick = () => {
            const { res: r } = ensureControls(article);
            verifyArticle(article, r, { silent: false });
        };
    }
    return { refreshBtn, verifyBtn, res, countdown };
}

// ---- Verification orchestration (UI) ----
async function verifyArticle(article, resEl, opts = {}) {
    const show = (ok, msg) =>
        setVerifyResult(resEl, ok, msg, { onlyUpdateOnChange: true });
    try {
        console.debug(
            "[LAP] verify start",
            article.dataset.lapUrl || article.id
        );
        if (!opts.silent) {
            setVerifyResult(resEl, true, "verifying…", {
                onlyUpdateOnChange: true,
            });
        }

        const contractErr = checkArticleContract(article);
        if (contractErr) {
            console.warn("[LAP] contract error", contractErr);
            return show(false, `${contractErr} (LA_CONTRACT)`);
        }

        const stapled = parseStapled(article);
        const shapeErr = (function () {
            // replicate shape check but allow expired to continue
            const p = stapled.payload || {};
            if (!p.url) return "missing payload.url";
            if (!p.attestation_url) return "missing payload.attestation_url";
            if (!p.hash) return "missing payload.hash";
            if (!stapled.sig) return "missing sig";
            if (!stapled.resource_key) return "missing resource_key";
            if (
                typeof p.iat !== "number" ||
                typeof p.exp !== "number" ||
                Number.isNaN(p.iat) ||
                Number.isNaN(p.exp)
            )
                return "missing iat/exp";
            return null;
        })();
        if (shapeErr && shapeErr !== "attestation expired") {
            console.warn("[LAP] shape error", shapeErr);
            return show(false, `${shapeErr} (LA_SHAPE)`);
        }

        const result = await verifyFragment({
            stapled,
            verifyBytes: () =>
                verifyCanonicalBytes(article, stapled.payload.hash),
            fetchRAImpl: fetchRAImpl,
            nowImpl: nowSecs,
            policy: "strict",
            // Provide last known GOOD timestamp for offline-fallback policy
            lastKnownAt:
                typeof article._lapLastGoodAt === "number"
                    ? article._lapLastGoodAt
                    : null,
        });

        const { refreshBtn, countdown } = ensureControls(article);
        if (!result.ok) {
            console.warn("[LAP] live verify failed", result);

            let msg = result.message || result.code || "verify failed";
            if (
                result.code === "LA_EXPIRED" &&
                typeof result.telemetry?.exp === "number"
            ) {
                msg += ` — expired at ${fmtTime(result.telemetry.exp)}`;
            } else if (
                result.code === "LA_IAT_IN_FUTURE" &&
                typeof result.telemetry?.iat === "number"
            ) {
                msg += ` — valid from ${fmtTime(result.telemetry.iat)}`;
            }
            if (result.code === "LA_HASH_DRIFT" && refreshBtn) {
                refreshBtn.style.display = "inline-flex";
                refreshBtn.onclick = () => swapWithLive(article, resEl);
            } else if (refreshBtn) {
                refreshBtn.style.display = "none";
                refreshBtn.onclick = null;
            }
            show(false, `${msg} (${result.code || "LA_ERROR"})`);
            return;
        }

        show(
            true,
            `${result.message || "verified"} (${result.code || "LA_OK"})`
        );
        console.debug(
            "[LAP] verified live",
            stapled?.payload?.url || "",
            result
        );
        const liveExp =
            (result.details &&
                typeof result.details.fresh_until === "number" &&
                result.details.fresh_until) ||
            (result.telemetry &&
                typeof result.telemetry.exp === "number" &&
                result.telemetry.exp) ||
            undefined;
        // Hide refresh button on success
        if (refreshBtn) {
            refreshBtn.style.display = "none";
            refreshBtn.onclick = null;
        }
        startCountdown(article, countdown, resEl, liveExp);
    } catch (err) {
        console.error("[LAP] verify exception", err);
        const stapled = (() => {
            try {
                return parseStapled(article);
            } catch {
                return null;
            }
        })();
        const hintExp = stapled?.payload?.exp;
        const hint =
            typeof hintExp === "number"
                ? ` (last known: expired at ${fmtTime(hintExp)})`
                : "";
        show(
            false,
            `${
                (err && err.message ? err.message : "verification error") + hint
            } (LA_EXCEPTION)`
        );
    }
}

function decorateAndWire(article) {
    const { res, countdown } = ensureControls(article);
    console.debug(
        "[LAP] decorate article",
        article.dataset.lapUrl || article.id
    );
    // Show immediate local result and countdown (if not expired yet)
    verifyLocal(article, res);
    startCountdown(article, countdown, res);
    // One-shot live verification
    verifyArticle(article, res, { silent: false });
}

async function verifyLocal(article, resEl) {
    const show = (ok, msg) =>
        setVerifyResult(resEl, ok, msg, { onlyUpdateOnChange: true });
    try {
        const contractErr = checkArticleContract(article);
        if (contractErr) return show(false, `${contractErr} (LOCAL)`);
        const stapled = parseStapled(article);
        const bytesErr = await verifyCanonicalBytes(
            article,
            stapled.payload.hash
        );
        if (bytesErr) return show(false, `${bytesErr} (LOCAL)`);
        show(true, "stapled ok (LOCAL)");
    } catch (err) {
        show(
            false,
            err && err.message
                ? `${err.message} (LOCAL)`
                : "local verify error (LOCAL)"
        );
    }
}

function startCountdown(article, el, resEl, overrideExp) {
    if (el && el._lapTimer) {
        clearInterval(el._lapTimer);
        el._lapTimer = null;
    }
    let exp = null;
    try {
        const stapled = parseStapled(article);
        if (
            stapled &&
            stapled.payload &&
            typeof stapled.payload.exp === "number"
        )
            exp = stapled.payload.exp;
    } catch (_) {}
    if (typeof overrideExp === "number") exp = overrideExp;
    if (!el) return;
    if (!exp) {
        el.textContent = "";
        return;
    }
    const render = () => {
        const now = nowSecs();
        let delta = Math.max(0, exp - now);
        el.textContent =
            delta > 60 ? `${Math.ceil(delta / 60)} m` : `${delta} s`;
        if (delta === 0) {
            clearInterval(el._lapTimer);
            el._lapTimer = null;
        }
    };
    render();
    el._lapTimer = setInterval(render, 1000);
}

// Polling has been removed for simplicity

// --- Refresh flow: fetch live fragment and swap in-place ---
async function swapWithLive(article, resEl) {
    try {
        const stapled = parseStapled(article);
        const baseUrl = new URL(stapled.payload.url, window.location.href);
        let newArticle = await fetchAndExtractArticle(baseUrl.toString());
        if (!newArticle) {
            const htmxUrl = new URL(baseUrl);
            if (!htmxUrl.pathname.endsWith("/")) htmxUrl.pathname += "/";
            htmxUrl.pathname += "index.htmx";
            newArticle = await fetchAndExtractArticle(htmxUrl.toString());
        }
        if (!newArticle) {
            const idxUrl = new URL(baseUrl);
            if (!idxUrl.pathname.endsWith("/")) idxUrl.pathname += "/";
            idxUrl.pathname += "index.html";
            newArticle = await fetchAndExtractArticle(idxUrl.toString());
        }
        if (!newArticle) throw new Error("live fragment not found");
        const parent = article.parentNode;
        parent.replaceChild(newArticle, article);
        decorateAndWire(newArticle);
        const { res } = ensureControls(newArticle);
        await verifyArticle(newArticle, res, { silent: false });
    } catch (e) {
        if (resEl) {
            resEl.textContent = "✖ refresh failed";
            resEl.style.color = "crimson";
            resEl.title = "refresh failed";
        }
    }
}

async function fetchAndExtractArticle(url) {
    try {
        const res = await fetch(url, {
            cache: "no-store",
            credentials: "omit",
        });
        if (!res.ok) return null;
        const html = await res.text();
        const parser = new DOMParser();
        const doc = parser.parseFromString(html, "text/html");
        return (
            doc.querySelector("#lap-article") ||
            doc.querySelector('article[data-lap-profile="fragment"]') ||
            null
        );
    } catch {
        return null;
    }
}

export function init(root = document) {
    const nodes = root.querySelectorAll('article[data-lap-profile="fragment"]');
    console.debug("[LAP] init fragments", nodes.length);
    nodes.forEach(decorateAndWire);
}

// Expose small API for non-module callers if needed
window.LAPVerifier = {
    init,
    verify: (article) => {
        const { res } = ensureControls(article);
        return verifyArticle(article, res, { silent: false });
    },
};

// Auto-run on DOM ready
if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => init());
} else {
    init();
}
