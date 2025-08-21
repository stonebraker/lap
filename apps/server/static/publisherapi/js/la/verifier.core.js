// Copyright 2025 Jason Stonebraker
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// verifier.core.js — DOM-free verification helpers
//
// This file contains the protocol-centric verification logic, with NO DOM or
// UI concerns. It is designed to be easy to unit-test and to reason about.
//
// Core protocol intent captured here:
// - The payload.hash attests to the resource BYTES. If the bytes are unchanged,
//   hash remains the same across time-window refreshes.
// - The payload time window (iat..exp) establishes freshness. It is expected to
//   roll forward over time, without changing the resource bytes.
// - The live attestation can rotate signature and time window without changing
//   hash/resource_key/kid. Verification therefore enforces stability of
//   url/attestation_url/hash/resource_key/kid, but DOES NOT require the
//   signature to match between stapled and live attestations.
// - The verifier must check that the live attestation is currently fresh.
//
// These functions are assembled so tests can: (a) provide a stapled attestation
// and canonical-bytes checker; (b) fetch a live attestation; (c) ensure the
// relationship between stapled and live surfaces the correct states and errors.

// Clock helper, dependency-injectable for tests
export const nowSecs = () => Math.floor(Date.now() / 1000);

// Fetch a Resource Attestation from its canonical endpoint.
// Why: The protocol requires a canonical JSON endpoint for the attestation.
// Headers: prefer lap+json; no-store to avoid stale caches; omit credentials.
export async function fetchRA(url) {
    const res = await fetch(url, {
        headers: { Accept: "application/lap+json, application/json;q=0.8" },
        cache: "no-store",
        credentials: "omit",
    });
    if (!res.ok) throw new Error(`fetch failed: ${res.status}`);
    return res.json();
}

// Compare a stapled attestation to a freshly fetched attestation.
// Enforces stability of identity + content across refresh windows.
// Returns null if OK, otherwise a structured drift error: { code, message, details }
export function compareStapledVsFetched(stapled, fetched) {
    const ps = stapled.payload || {};
    const pf = (fetched && fetched.payload) || {};
    if (pf.url !== ps.url)
        return {
            code: "LA_URL_DRIFT",
            message: "payload.url mismatch (live vs stapled)",
            details: { expected: ps.url, actual: pf.url },
        };
    if (pf.attestation_url !== ps.attestation_url)
        return {
            code: "LA_ATTESTATION_URL_DRIFT",
            message: "attestation_url mismatch (live vs stapled)",
            details: {
                expected: ps.attestation_url,
                actual: pf.attestation_url,
            },
        };
    if ((pf.hash || "") !== (ps.hash || ""))
        return {
            code: "LA_HASH_DRIFT",
            message: "hash mismatch (live vs stapled)",
            details: {
                expected_sha256: (ps.hash || "").replace(/^sha256:/, ""),
                actual_sha256: (pf.hash || "").replace(/^sha256:/, ""),
            },
        };
    // Do not require signature equality across refresh windows; allow rotation
    if ((fetched.resource_key || "") !== (stapled.resource_key || ""))
        return {
            code: "LA_RESOURCE_KEY_DRIFT",
            message: "resource_key mismatch (live vs stapled)",
            details: {
                expected: stapled.resource_key || "",
                actual: fetched.resource_key || "",
            },
        };
    if ((pf.kid || "") !== (ps.kid || ""))
        return {
            code: "LA_KID_DRIFT",
            message: "kid mismatch (live vs stapled)",
            details: { expected: ps.kid || "", actual: pf.kid || "" },
        };
    return null;
}

// Evaluate a time window against a clock.
// Returns one of: 'fresh' | 'expired' | 'notyet'
export function evaluateWindow(p, now = nowSecs()) {
    if (typeof p.iat === "number" && now < p.iat) return "notyet";
    if (typeof p.exp === "number" && now > p.exp) return "expired";
    return "fresh";
}

// Build a canonical JSON string of the payload in stable key order for signing
function canonicalizePayloadForSignature(payload) {
    const order = [
        "url",
        "attestation_url",
        "hash",
        "etag",
        "iat",
        "exp",
        "kid",
    ];
    const obj = {};
    for (const k of order) {
        if (Object.prototype.hasOwnProperty.call(payload || {}, k)) {
            obj[k] = payload[k];
        }
    }
    return JSON.stringify(obj);
}

async function verifySignatureSchnorr(stapled, schnorrImpl) {
    try {
        const payload = stapled && stapled.payload ? stapled.payload : null;
        const sigHex = (stapled && stapled.sig) || "";
        const pubHex = (stapled && stapled.resource_key) || "";
        if (!payload || !sigHex || !pubHex) return false;
        const json = canonicalizePayloadForSignature(payload);
        const enc = new TextEncoder();
        const digest = await crypto.subtle.digest("SHA-256", enc.encode(json));
        const msg = new Uint8Array(digest);

        // Prefer injected implementation
        if (schnorrImpl && typeof schnorrImpl.verify === "function") {
            return await schnorrImpl.verify(sigHex, msg, pubHex);
        }

        // Try global NobleSecp256k1 (loaded via script tag)
        if (
            typeof globalThis.NobleSecp256k1 !== "undefined" &&
            globalThis.NobleSecp256k1.schnorr
        ) {
            return await globalThis.NobleSecp256k1.schnorr.verify(
                sigHex,
                msg,
                pubHex
            );
        }

        // Fallback to dynamic import
        const impl = (await import("../vendor/noble-secp256k1.js")).schnorr;
        return await impl.verify(sigHex, msg, pubHex);
    } catch (_) {
        return false;
    }
}

// Orchestrated verification given DOM-free inputs
// - stapled: { payload, resource_key, sig }
// - verifyBytes: async () => null | errorCode (UI supplies canonical-bytes check)
// - fetchRAImpl, nowImpl: dependency-injected for testing
// Orchestrated verification flow for a fragment:
// Steps:
//  1) Verify canonical bytes (UI supplies function to compute sha256 over bytes
//     referenced by the bytes link). Detects content tampering.
//  2) Fetch the live attestation from payload.attestation_url.
//  3) Compare stapled vs fetched enforcing stability of identity + content.
//  4) Ensure the fetched attestation is fresh (now within iat..exp).
// Returns: { ok: boolean, reason?: string, live?: Attestation }
// Rationale: This isolates protocol checks so UI can render/status without
// embedding policy.
export async function verifyFragment({
    stapled,
    verifyBytes,
    fetchRAImpl = fetchRA,
    nowImpl = nowSecs,
    policy = "strict",
    skewSeconds = 120,
    lastKnownAt = null,
    verifySigImpl = null,
}) {
    const now = nowImpl();
    const baseTelemetry = (p) => ({
        url: p && p.url ? p.url : null,
        kid: p && p.kid ? p.kid : null,
        iat: p && typeof p.iat === "number" ? p.iat : null,
        exp: p && typeof p.exp === "number" ? p.exp : null,
        now,
        policy,
    });

    // Stapled presence
    if (!stapled) {
        return {
            ok: false,
            status: "error",
            code: "LA_NO_ATTESTATION",
            message: "no attestation provided",
            telemetry: baseTelemetry(null),
        };
    }
    if (!stapled.payload) {
        return {
            ok: false,
            status: "error",
            code: "LA_ATTESTATION_MALFORMED",
            message: "malformed stapled object",
            telemetry: baseTelemetry(null),
        };
    }

    // Signature verification MUST precede other checks
    const sigOk = await verifySignatureSchnorr(stapled, verifySigImpl);
    if (!sigOk) {
        return {
            ok: false,
            status: "error",
            code: "LA_SIG_INVALID",
            message: "signature verification failed",
            details: { kid: stapled?.payload?.kid || null },
            telemetry: baseTelemetry(stapled.payload),
        };
    }

    // Bytes verification (caller-supplied)
    const bytesErr = await verifyBytes();
    if (bytesErr) {
        return {
            ok: false,
            status: "error",
            code: "LA_HASH_MISMATCH",
            message: String(bytesErr),
            telemetry: baseTelemetry(stapled.payload),
        };
    }

    // Fetch live attestation and map transport/parse errors
    let live;
    try {
        live = await fetchRAImpl(stapled.payload.attestation_url);
    } catch (err) {
        // Offline handling
        if (policy === "offline-fallback" && typeof lastKnownAt === "number") {
            return {
                ok: true,
                status: "warn",
                code: "LA_OFFLINE_STALE",
                message: "offline; showing last known good result",
                details: { cached_at: lastKnownAt },
                telemetry: baseTelemetry(stapled.payload),
            };
        }
        const msg =
            err && err.message ? String(err.message) : "fetch/parse failed";
        const m = /fetch failed: (\d+)/.exec(msg);
        if (m) {
            return {
                ok: false,
                status: "error",
                code: "LA_FETCH_FAILED",
                message: msg,
                details: {
                    http_status: Number(m[1]),
                    phase: "bundle",
                },
                telemetry: baseTelemetry(stapled.payload),
            };
        }
        return {
            ok: false,
            status: "error",
            code: "LA_ATTESTATION_MALFORMED",
            message: msg,
            telemetry: baseTelemetry(stapled.payload),
        };
    }

    // Cross-attestation drift checks
    const drift = compareStapledVsFetched(stapled, live);
    if (drift) {
        return {
            ok: false,
            status: "error",
            code: drift.code,
            message: drift.message,
            details: drift.details,
            telemetry: baseTelemetry(stapled.payload),
        };
    }

    // Freshness checks with policy handling
    const state = evaluateWindow(live.payload, now);
    if (state === "notyet") {
        // auto-refresh may recover from notyet
        if (policy === "auto-refresh") {
            try {
                const refetched = await fetchRAImpl(
                    stapled.payload.attestation_url
                );
                const s = evaluateWindow(
                    (refetched && refetched.payload) || {},
                    nowImpl()
                );
                if (s === "fresh") {
                    return {
                        ok: true,
                        status: "ok",
                        code: "LA_OK",
                        message: "verified",
                        details: { fresh_until: refetched.payload.exp ?? null },
                        telemetry: baseTelemetry(refetched.payload),
                    };
                }
                return {
                    ok: false,
                    status: "error",
                    code: "LA_EXPIRED_AFTER_REFRESH",
                    message: "attestation not yet valid after refresh",
                    telemetry: baseTelemetry(refetched.payload || live.payload),
                };
            } catch (_) {
                // fall through to error
            }
        }
        return {
            ok: false,
            status: "error",
            code: "LA_IAT_IN_FUTURE",
            message: "live attestation not yet valid",
            telemetry: baseTelemetry(live.payload),
        };
    }
    if (state === "expired") {
        // graceful acceptance if within skew window
        const exp =
            typeof live.payload.exp === "number" ? live.payload.exp : null;
        const skew = typeof skewSeconds === "number" ? skewSeconds : 0;
        if (policy === "graceful" && exp !== null && now <= exp + skew) {
            return {
                ok: true,
                status: "warn",
                code: "LA_EXPIRED_GRACE",
                message: "attestation slightly expired (grace)",
                details: { expired_at: exp, skew },
                telemetry: baseTelemetry(live.payload),
            };
        }
        if (policy === "auto-refresh") {
            try {
                const refetched = await fetchRAImpl(
                    stapled.payload.attestation_url
                );
                const s = evaluateWindow(
                    (refetched && refetched.payload) || {},
                    nowImpl()
                );
                if (s === "fresh") {
                    return {
                        ok: true,
                        status: "ok",
                        code: "LA_OK",
                        message: "verified",
                        details: { fresh_until: refetched.payload.exp ?? null },
                        telemetry: baseTelemetry(refetched.payload),
                    };
                }
                return {
                    ok: false,
                    status: "error",
                    code: "LA_EXPIRED_AFTER_REFRESH",
                    message: "attestation expired after refresh",
                    telemetry: baseTelemetry(refetched.payload || live.payload),
                };
            } catch (_) {
                // fall through
            }
        }
        return {
            ok: false,
            status: "error",
            code: "LA_EXPIRED",
            message: "live attestation expired",
            telemetry: baseTelemetry(live.payload),
        };
    }

    // Success
    return {
        ok: true,
        status: "ok",
        code: "LA_OK",
        message: "verified",
        details: {
            fresh_until: live && live.payload ? live.payload.exp : null,
        },
        telemetry: baseTelemetry(live.payload),
    };
}
