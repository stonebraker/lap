/**
 * Global script version of noble-secp256k1
 * Loads the vendored library and exposes it globally
 */

(async function () {
    try {
        // Load the vendored noble-secp256k1 as a module and expose globally
        const module = await import("./noble-secp256k1.js");

        window.NobleSecp256k1 = {
            schnorr: module.schnorr,
        };

        console.log("NobleSecp256k1 loaded successfully from vendored library");

        // Dispatch an event so pages can know when it's ready
        window.dispatchEvent(new CustomEvent("noble-loaded"));
    } catch (error) {
        console.error("Failed to load vendored noble-secp256k1:", error);

        // Provide a fallback that clearly indicates failure
        window.NobleSecp256k1 = {
            schnorr: {
                async verify() {
                    throw new Error("Noble secp256k1 failed to load");
                },
            },
        };
    }
})();
