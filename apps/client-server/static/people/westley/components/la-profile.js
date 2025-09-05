class LaProfile extends HTMLElement {
    constructor() {
        super();
        this.attachShadow({ mode: "open" });
        this.fragmentHTML = null; // Store the complete fragment for verification
    }

    async connectedCallback() {
        console.log("la-profile: connectedCallback called");
        const profileUrl = this.getAttribute("src");
        console.log("la-profile: profileUrl =", profileUrl);

        if (!profileUrl) {
            console.error("la-profile: src attribute is required");
            return;
        }

        try {
            console.log("la-profile: fetching profile from", profileUrl);
            const response = await fetch(profileUrl);
            const html = await response.text();
            console.log("la-profile: fetched HTML length", html.length);

            // Store the complete fragment HTML for verification
            this.fragmentHTML = html;

            // Parse the HTML to extract data attributes
            const parser = new DOMParser();
            const doc = parser.parseFromString(html, "text/html");
            const article = doc.querySelector("article");

            if (!article) {
                throw new Error("No article element found in profile HTML");
            }

            // Extract data from attributes
            const displayName =
                article
                    .querySelector("[data-profile-display-name]")
                    ?.getAttribute("data-profile-display-name") ||
                "Alice Cooks";
            const picture =
                article
                    .querySelector("[data-profile-picture]")
                    ?.getAttribute("data-profile-picture") ||
                "http://localhost:8080/people/alice/images/alice_profile_pic.jpg";
            const website =
                article
                    .querySelector("[data-profile-website]")
                    ?.getAttribute("data-profile-website") ||
                "http://localhost:8080/people/alice";
            const banner =
                article
                    .querySelector("[data-profile-banner]")
                    ?.getAttribute("data-profile-banner") ||
                "http://localhost:8080/people/alice/alice_banner.jpg";
            const about =
                article
                    .querySelector("[data-profile-about]")
                    ?.getAttribute("data-profile-about") ||
                "Guest Chef at The Fireswamp Inn";

            console.log("la-profile: extracted about =", about);
            console.log(
                "la-profile: about length =",
                about ? about.length : "null"
            );

            this.shadowRoot.innerHTML = `
                <style>
                    :host {
                        display: block;
                        font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji";
                    }

                    .profile-card {
                        position: relative;
                        margin-bottom: 1.5rem;
                        border-radius: 0.5rem;
                        overflow: hidden;
                    }

                    .banner-container {
                        position: relative;
                        height: 8rem;
                        background: linear-gradient(to right, #78350f, #9a3412);
                    }

                    .banner-image {
                        width: 100%;
                        height: 100%;
                        object-fit: cover;
                        object-position: bottom;
                        opacity: 0.8;
                    }

                    .banner-overlay {
                        position: absolute;
                        top: 0;
                        left: 0;
                        right: 0;
                        bottom: 0;
                        background-color: rgba(0, 0, 0, 0.4);
                    }

                    .profile-overlay {
                        position: absolute;
                        top: 0;
                        bottom: 0;
                        left: 0;
                        right: 0;
                        padding: 1rem;
                    }

                    .profile-layout {
                        display: flex;
                        align-items: flex-end;
                        gap: 1rem;
                    }

                    .profile-picture {
                        width: 4rem;
                        height: 4rem;
                        border-radius: 50%;
                        border: 4px solid #fde68a;
                        object-fit: cover;
                        flex-shrink: 0;
                        box-sizing: border-box;
                    }

                    .profile-info {
                        flex: 1;
                        min-width: 0;
                    }

                    .profile-name {
                        font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji";
                        font-size: 1.25rem;
                        font-weight: 700;
                        color: white;
                        margin: 0 0 0.25rem 0;
                    }

                    .profile-role {
                        font-size: 0.875rem;
                        color: #fde68a;
                        margin: 0 0 0.5rem 0;
                        font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji";
                    }

                    .profile-website {
                        font-size: 0.875rem;
                        color: #fbbf24;
                        text-decoration: none;
                        margin: 0;
                        font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji";
                    }

                    .profile-website:hover {
                        color: #f59e0b;
                    }

                    .action-buttons {
                        position: absolute;
                        top: 0.25rem;
                        right: 0.25rem;
                        display: flex;
                        align-items: center;
                        gap: 0.5rem;
                        z-index: 10;
                        opacity: 0;
                        transition: opacity 0.2s ease-in-out;
                    }

                    .profile-card:hover .action-buttons {
                        opacity: 1;
                    }

                    .action-button {
                        display: flex;
                        align-items: center;
                        justify-content: center;
                        background-color: rgba(55, 65, 81, 0.6);
                        border: 1px solid rgba(156, 163, 175, 0.3);
                        color: #9ca3af;
                        padding: 0.125rem 0.5rem;
                        border-radius: 9999px;
                        font-size: 0.625rem;
                        font-weight: 500;
                        cursor: pointer;
                        transition: all 0.2s;
                        backdrop-filter: blur(8px);
                        min-width: 3rem;
                        white-space: nowrap;
                    }

                    .action-button:hover {
                        color: #d1d5db;
                        border-color: rgba(156, 163, 175, 0.5);
                    }

                    .action-button:disabled {
                        opacity: 0.5;
                        cursor: not-allowed;
                    }

                    .action-button.verifying {
                        color: #fbbf24;
                    }

                    .action-button.verified {
                        color: #4ade80;
                        border-color: #4ade80;
                    }

                    .action-button.failed {
                        color: #ef4444;
                        border-color: #ef4444;
                    }

                    .action-icon {
                        width: 0.75rem;
                        height: 0.75rem;
                        margin-right: 0.125rem;
                    }

                    /* Style the imported content */
                    article {
                        margin: 0;
                        padding: 0;
                    }

                    header {
                        display: none;
                    }

                    .banner {
                        display: none;
                    }

                    .profile-pic {
                        display: none;
                    }

                    h1 {
                        display: none;
                    }

                    /* Hide original fragment content */
                    .banner p,
                    header p,
                    .stats p,
                    .about p {
                        display: none;
                    }

                    a {
                        display: none;
                    }

                    .stats {
                        display: none;
                    }

                    .about {
                        display: none;
                    }
                </style>
                <div class="profile-card">
                    <div class="banner-container">
                        <img class="banner-image" src="${banner}" alt="${displayName}'s banner" />
                        <div class="banner-overlay"></div>
                    </div>
                    <div class="profile-overlay">
                        <div class="profile-layout">
                            <img class="profile-picture" src="${picture}" alt="${displayName}'s profile picture" />
                            <div class="profile-info">
                                <h3 class="profile-name">${displayName}</h3>
                                <p class="profile-role">${about}</p>
                                <a href="${website}" class="profile-website">${website}</a>
                            </div>
                            <div class="action-buttons">
                                <button class="action-button" onclick="this.copyFragment()">
                                    <svg class="action-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
                                    </svg>
                                    <span>Copy</span>
                                </button>
                                <button class="action-button" onclick="this.verifyFragment()">
                                    <svg class="action-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                                    </svg>
                                    <span>Verify</span>
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            `;

            // Add methods to the buttons
            const copyButton = this.shadowRoot.querySelector(".action-button");
            const verifyButton =
                this.shadowRoot.querySelectorAll(".action-button")[1];

            copyButton.copyFragment = () => this.copyFragment(copyButton);
            verifyButton.verifyFragment = () =>
                this.verifyFragment(verifyButton);

            console.log("la-profile: component rendered successfully");
        } catch (error) {
            console.error("la-profile: Failed to load profile", error);
            this.shadowRoot.innerHTML = `
                <div style="color: #ef4444; padding: 1rem; border: 1px solid #ef4444; border-radius: 0.5rem;">
                    Failed to load profile: ${error.message}
                </div>
            `;
        }
    }

    copyFragment(button) {
        if (!this.fragmentHTML) {
            console.error("No fragment HTML available for copying");
            return;
        }

        const span = button.querySelector("span");
        const originalText = span.textContent;

        // Copy to clipboard
        if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard
                .writeText(this.fragmentHTML)
                .then(() => {
                    span.textContent = "Copied";
                    setTimeout(() => {
                        span.textContent = originalText;
                    }, 2000);
                })
                .catch((err) => {
                    console.error("Failed to copy: ", err);
                    this.fallbackCopy(this.fragmentHTML, button);
                });
        } else {
            this.fallbackCopy(this.fragmentHTML, button);
        }
    }

    fallbackCopy(text, button) {
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
                const span = button.querySelector("span");
                const originalText = span.textContent;
                span.textContent = "Copied";
                setTimeout(() => {
                    span.textContent = originalText;
                }, 2000);
            }
        } catch (err) {
            console.error("Fallback copy failed: ", err);
        }

        document.body.removeChild(textArea);
    }

    async verifyFragment(button) {
        if (!this.fragmentHTML) {
            console.error("No fragment HTML available for verification");
            return;
        }

        const span = button.querySelector("span");
        const originalText = span.textContent;
        const originalIcon = button.querySelector("svg");

        // Disable button during verification
        button.disabled = true;

        try {
            // Send to verification service
            const response = await fetch("http://localhost:8082/verify", {
                method: "POST",
                headers: {
                    "Content-Type": "text/html",
                    "X-Fetch-URL":
                        "http://localhost:8080/people/alice/profile/index.htmx",
                },
                body: this.fragmentHTML,
            });

            const result = await response.json();

            if (result.verified) {
                // Success - show checkmark
                span.textContent = "Verified";
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
                span.textContent = "Failed";
                button.classList.add("failed");

                // Change icon to X
                originalIcon.innerHTML = `
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                `;

                // Reset after 3 seconds
                setTimeout(() => {
                    span.textContent = "Verify";
                    button.classList.remove("failed");
                    originalIcon.innerHTML = `
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                    `;
                }, 3000);
            }
        } catch (error) {
            console.error("Verification error:", error);

            // Show error state
            span.textContent = "Error";
            button.classList.add("failed");

            // Change icon to X
            originalIcon.innerHTML = `
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
            `;

            // Reset after 3 seconds
            setTimeout(() => {
                span.textContent = "Verify";
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
}

customElements.define("la-profile", LaProfile);
