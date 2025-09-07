/**
 * LAP Fragment Verifier
 * A JavaScript library that finds LAP fragments in the page and displays copy and verify buttons on hover
 */

class LAPFragmentVerifier {
    constructor(options = {}) {
        this.options = {
            verificationServiceUrl: "http://localhost:8082/verify",
            copyButtonText: "Copy",
            verifyButtonText: "Verify",
            copiedText: "Copied",
            verifiedText: "Verified",
            failedText: "Failed",
            errorText: "Error",
            hoverDelay: 0,
            ...options,
        };

        this.fragments = new Map();
        this.init();
    }

    init() {
        // Wait for DOM to be ready
        if (document.readyState === "loading") {
            document.addEventListener("DOMContentLoaded", () =>
                this.scanForFragments()
            );
        } else {
            this.scanForFragments();
        }

        // Set up generic content change detection
        this.setupContentChangeDetection();
    }

    setupContentChangeDetection() {
        // Use MutationObserver to detect when new content is added to the DOM
        const observer = new MutationObserver((mutations) => {
            let shouldRescan = false;

            mutations.forEach((mutation) => {
                // Check if new nodes were added
                if (
                    mutation.type === "childList" &&
                    mutation.addedNodes.length > 0
                ) {
                    // Check if any added nodes contain LAP fragments
                    mutation.addedNodes.forEach((node) => {
                        if (node.nodeType === Node.ELEMENT_NODE) {
                            // Check if the node itself is a LAP fragment
                            if (
                                node.matches &&
                                node.matches("article[data-la-spec]")
                            ) {
                                shouldRescan = true;
                            }
                            // Check if the node contains LAP fragments
                            if (
                                node.querySelector &&
                                node.querySelector("article[data-la-spec]")
                            ) {
                                shouldRescan = true;
                            }
                        }
                    });
                }
            });

            if (shouldRescan) {
                // Small delay to ensure DOM is fully updated
                setTimeout(() => {
                    this.scanForFragments();
                }, 100);
            }
        });

        // Start observing the entire document for changes
        observer.observe(document.body, {
            childList: true,
            subtree: true,
        });

        // Store observer reference for cleanup
        this.mutationObserver = observer;
    }

    scanForFragments() {
        // Find all LAP fragments (articles with data-la-spec attribute)
        const fragmentElements = document.querySelectorAll(
            "article[data-la-spec]"
        );

        let newFragments = 0;
        fragmentElements.forEach((fragment, index) => {
            // Check if this fragment is already being tracked
            const isAlreadyTracked = Array.from(this.fragments.values()).some(
                (frag) => frag.element === fragment
            );

            if (!isAlreadyTracked) {
                this.attachFragmentControls(fragment, index);
                newFragments++;
            }
        });

        if (newFragments > 0) {
            console.log(
                `LAP Fragment Verifier: Found ${newFragments} new fragments (${fragmentElements.length} total)`
            );
        }
    }

    attachFragmentControls(fragment, index) {
        // Create a wrapper div for the fragment if it doesn't exist
        let wrapper = fragment.parentElement;
        if (!wrapper.classList.contains("lap-fragment-wrapper")) {
            wrapper = document.createElement("div");
            wrapper.className = "lap-fragment-wrapper";
            fragment.parentNode.insertBefore(wrapper, fragment);
            wrapper.appendChild(fragment);
        }

        // Add hover controls
        this.addHoverControls(wrapper, fragment, index);
    }

    addHoverControls(wrapper, fragment, index) {
        // Create the action buttons container
        const actionButtons = document.createElement("div");
        actionButtons.className = "lap-action-buttons";
        actionButtons.innerHTML = `
            <button class="lap-copy-button" data-fragment-index="${index}">
                <svg class="lap-button-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
                </svg>
                <span>${this.options.copyButtonText}</span>
            </button>
            <button class="lap-verify-button" data-fragment-index="${index}">
                <svg class="lap-button-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                </svg>
                <span>${this.options.verifyButtonText}</span>
            </button>
        `;

        // Add CSS styles
        this.addStyles();

        // Add the buttons to the wrapper
        wrapper.appendChild(actionButtons);

        // Store fragment reference
        this.fragments.set(index, {
            element: fragment,
            wrapper: wrapper,
            actionButtons: actionButtons,
        });

        // Add event listeners
        this.addEventListeners(wrapper, fragment, index);
    }

    addEventListeners(wrapper, fragment, index) {
        let hoverTimeout;

        // Show buttons on hover
        wrapper.addEventListener("mouseenter", () => {
            clearTimeout(hoverTimeout);
            hoverTimeout = setTimeout(() => {
                wrapper.classList.add("lap-hover");
            }, this.options.hoverDelay);
        });

        // Hide buttons when not hovering
        wrapper.addEventListener("mouseleave", () => {
            clearTimeout(hoverTimeout);
            wrapper.classList.remove("lap-hover");
        });

        // Copy button
        const copyButton = wrapper.querySelector(".lap-copy-button");
        copyButton.addEventListener("click", (e) => {
            e.preventDefault();
            e.stopPropagation();
            this.copyFragment(fragment, copyButton);
        });

        // Verify button
        const verifyButton = wrapper.querySelector(".lap-verify-button");
        verifyButton.addEventListener("click", (e) => {
            e.preventDefault();
            e.stopPropagation();
            this.verifyFragment(fragment, verifyButton);
        });
    }

    addStyles() {
        // Only add styles once
        if (document.getElementById("lap-fragment-verifier-styles")) {
            return;
        }

        const style = document.createElement("style");
        style.id = "lap-fragment-verifier-styles";
        style.textContent = `
            .lap-fragment-wrapper {
                position: relative;
                display: block;
                width: 100%;
            }

            .lap-action-buttons {
                position: absolute;
                top: 0.5rem;
                right: 0.5rem;
                display: flex;
                align-items: center;
                gap: 0.5rem;
                z-index: 10;
                opacity: 0;
                transition: opacity 0.3s ease-in-out;
                pointer-events: none;
            }

            .lap-fragment-wrapper.lap-hover .lap-action-buttons {
                opacity: 1;
                pointer-events: auto;
            }

            .lap-copy-button,
            .lap-verify-button {
                display: flex;
                align-items: center;
                justify-content: center;
                background-color: rgba(55, 65, 81, 0.8);
                border: 1px solid rgba(156, 163, 175, 0.3);
                color: #9ca3af;
                padding: 0.25rem 0.5rem;
                border-radius: 9999px;
                font-size: 0.75rem;
                font-weight: 500;
                cursor: pointer;
                transition: all 0.2s;
                backdrop-filter: blur(8px);
                min-width: 3.5rem;
                white-space: nowrap;
                gap: 0.25rem;
            }

            .lap-copy-button:hover,
            .lap-verify-button:hover {
                color: #d1d5db;
                border-color: rgba(156, 163, 175, 0.5);
                background-color: rgba(55, 65, 81, 0.9);
            }

            .lap-copy-button:disabled,
            .lap-verify-button:disabled {
                opacity: 0.5;
                cursor: not-allowed;
            }

            .lap-button-icon {
                width: 0.875rem;
                height: 0.875rem;
            }

            /* State classes */
            .lap-copy-button.copied {
                color: #4ade80;
                border-color: #4ade80;
            }

            .lap-verify-button.verifying {
                color: #fbbf24;
                border-color: #fbbf24;
            }

            .lap-verify-button.verified {
                color: #4ade80;
                border-color: #4ade80;
            }

            .lap-verify-button.failed {
                color: #ef4444;
                border-color: #ef4444;
            }
        `;

        document.head.appendChild(style);
    }

    copyFragment(fragment, button) {
        const span = button.querySelector("span");
        const originalText = span.textContent;
        const originalIcon = button.querySelector("svg");

        // Get the complete fragment HTML
        const fragmentHTML = fragment.outerHTML;

        // Copy to clipboard
        if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard
                .writeText(fragmentHTML)
                .then(() => {
                    this.showCopied(button, span, originalText, originalIcon);
                })
                .catch((err) => {
                    console.error("Failed to copy: ", err);
                    this.fallbackCopy(
                        fragmentHTML,
                        button,
                        span,
                        originalText,
                        originalIcon
                    );
                });
        } else {
            this.fallbackCopy(
                fragmentHTML,
                button,
                span,
                originalText,
                originalIcon
            );
        }
    }

    fallbackCopy(text, button, span, originalText, originalIcon) {
        const textArea = document.createElement("textarea");
        textArea.value = text;
        textArea.style.position = "fixed";
        textArea.style.left = "-999999px";
        textArea.style.top = "-999999px";
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();

        try {
            const successful = document.execCommand("copy");
            if (successful) {
                this.showCopied(button, span, originalText, originalIcon);
            }
        } catch (err) {
            console.error("Fallback copy failed: ", err);
        }

        document.body.removeChild(textArea);
    }

    showCopied(button, span, originalText, originalIcon) {
        span.textContent = this.options.copiedText;
        button.classList.add("copied");

        // Change icon to checkmark
        originalIcon.innerHTML = `
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
        `;

        // Reset after 2 seconds
        setTimeout(() => {
            span.textContent = originalText;
            button.classList.remove("copied");
            originalIcon.innerHTML = `
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
            `;
        }, 2000);
    }

    async verifyFragment(fragment, button) {
        const span = button.querySelector("span");
        const originalText = span.textContent;
        const originalIcon = button.querySelector("svg");

        // Disable button during verification
        button.disabled = true;
        button.classList.add("verifying");

        try {
            // Get the complete fragment HTML
            const fragmentHTML = fragment.outerHTML;

            // Get the fragment URL for verification
            const fragmentUrl =
                fragment.getAttribute("data-la-fragment-url") ||
                fragment
                    .querySelector("[data-la-fragment-url]")
                    ?.getAttribute("data-la-fragment-url");

            // Send to verification service
            const response = await fetch(this.options.verificationServiceUrl, {
                method: "POST",
                headers: {
                    "Content-Type": "text/html",
                    "X-Fetch-URL": fragmentUrl || window.location.href,
                },
                body: fragmentHTML,
            });

            const result = await response.json();

            if (result.verified) {
                // Success - show checkmark
                span.textContent = this.options.verifiedText;
                button.classList.remove("verifying");
                button.classList.add("verified");

                // Change icon to checkmark
                originalIcon.innerHTML = `
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
                `;

                // Reset after 3 seconds
                setTimeout(() => {
                    span.textContent = originalText;
                    button.classList.remove("verified");
                    originalIcon.innerHTML = `
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                    `;
                }, 3000);
            } else {
                // Verification failed
                span.textContent = this.options.failedText;
                button.classList.remove("verifying");
                button.classList.add("failed");

                // Change icon to X
                originalIcon.innerHTML = `
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                `;

                // Reset after 3 seconds
                setTimeout(() => {
                    span.textContent = originalText;
                    button.classList.remove("failed");
                    originalIcon.innerHTML = `
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                    `;
                }, 3000);
            }
        } catch (error) {
            console.error("Verification error:", error);

            // Show error state
            span.textContent = this.options.errorText;
            button.classList.remove("verifying");
            button.classList.add("failed");

            // Change icon to X
            originalIcon.innerHTML = `
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
            `;

            // Reset after 3 seconds
            setTimeout(() => {
                span.textContent = originalText;
                button.classList.remove("failed");
                originalIcon.innerHTML = `
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                `;
            }, 3000);
        } finally {
            // Re-enable button
            button.disabled = false;
        }
    }

    // Public API methods
    refresh() {
        this.scanForFragments();
    }

    getFragments() {
        return Array.from(this.fragments.values());
    }

    destroy() {
        // Remove all event listeners and clean up
        this.fragments.forEach(({ wrapper }) => {
            wrapper.classList.remove("lap-hover");
        });
        this.fragments.clear();

        // Disconnect the mutation observer
        if (this.mutationObserver) {
            this.mutationObserver.disconnect();
            this.mutationObserver = null;
        }
    }
}

// Auto-initialize when script loads
if (typeof window !== "undefined") {
    window.LAPFragmentVerifier = LAPFragmentVerifier;

    // Auto-initialize with default options
    document.addEventListener("DOMContentLoaded", () => {
        window.lapVerifier = new LAPFragmentVerifier();
    });
}

// Export for module systems
if (typeof module !== "undefined" && module.exports) {
    module.exports = LAPFragmentVerifier;
}
