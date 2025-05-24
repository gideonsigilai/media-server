// Theme management
class ThemeManager {
    constructor() {
        this.theme = localStorage.getItem('theme') || 'light';
        this.init();
    }

    init() {
        this.applyTheme();
        this.bindEvents();
    }

    applyTheme() {
        document.documentElement.setAttribute('data-theme', this.theme);
        this.updateThemeIcon();
    }

    updateThemeIcon() {
        const themeIcon = document.querySelector('.theme-icon');
        if (themeIcon) {
            themeIcon.textContent = this.theme === 'dark' ? '☀️' : '🌙';
        }
    }

    toggleTheme() {
        this.theme = this.theme === 'light' ? 'dark' : 'light';
        localStorage.setItem('theme', this.theme);
        this.applyTheme();
    }

    bindEvents() {
        const themeToggle = document.getElementById('theme-toggle');
        if (themeToggle) {
            themeToggle.addEventListener('click', () => this.toggleTheme());
        }
    }
}

// File browser enhancements
class FileBrowser {
    constructor() {
        this.init();
    }

    init() {
        this.addKeyboardNavigation();
        this.addLoadingStates();
        this.addFilePreview();
    }

    addKeyboardNavigation() {
        document.addEventListener('keydown', (e) => {
            // Navigate back with Escape key
            if (e.key === 'Escape') {
                const backButton = document.querySelector('.breadcrumb-link[href="/"]');
                if (backButton && window.location.pathname !== '/') {
                    window.history.back();
                }
            }

            // Navigate with arrow keys
            if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
                this.navigateWithArrows(e.key === 'ArrowDown');
                e.preventDefault();
            }

            // Enter to open selected item
            if (e.key === 'Enter') {
                const focused = document.activeElement;
                if (focused && focused.classList.contains('file-link')) {
                    focused.click();
                }
            }
        });
    }

    navigateWithArrows(down) {
        const fileLinks = Array.from(document.querySelectorAll('.file-link'));
        const currentIndex = fileLinks.indexOf(document.activeElement);
        
        let nextIndex;
        if (currentIndex === -1) {
            nextIndex = down ? 0 : fileLinks.length - 1;
        } else {
            nextIndex = down ? 
                Math.min(currentIndex + 1, fileLinks.length - 1) :
                Math.max(currentIndex - 1, 0);
        }
        
        if (fileLinks[nextIndex]) {
            fileLinks[nextIndex].focus();
        }
    }

    addLoadingStates() {
        document.addEventListener('click', (e) => {
            const fileLink = e.target.closest('.file-link');
            if (fileLink) {
                const fileItem = fileLink.closest('.file-item');
                fileItem.style.opacity = '0.6';
                fileItem.style.pointerEvents = 'none';
                
                // Reset after a delay (in case navigation fails)
                setTimeout(() => {
                    fileItem.style.opacity = '';
                    fileItem.style.pointerEvents = '';
                }, 3000);
            }
        });
    }

    addFilePreview() {
        // Add hover effects and tooltips for better UX
        const fileItems = document.querySelectorAll('.file-item');
        fileItems.forEach(item => {
            const fileName = item.querySelector('.file-name');
            if (fileName && fileName.scrollWidth > fileName.clientWidth) {
                fileName.title = fileName.textContent;
            }
        });
    }
}

// Accessibility enhancements
class AccessibilityManager {
    constructor() {
        this.init();
    }

    init() {
        this.addAriaLabels();
        this.addFocusManagement();
        this.addSkipLinks();
    }

    addAriaLabels() {
        // Add aria-labels to file items
        const fileItems = document.querySelectorAll('.file-item');
        fileItems.forEach(item => {
            const link = item.querySelector('.file-link');
            const fileName = item.querySelector('.file-name');
            const isDirectory = item.classList.contains('file-item-directory');
            const isMedia = item.classList.contains('file-item-media');
            
            if (link && fileName) {
                let label = fileName.textContent;
                if (isDirectory) {
                    label += ', directory';
                } else if (isMedia) {
                    label += ', media file';
                } else {
                    label += ', file';
                }
                link.setAttribute('aria-label', label);
            }
        });
    }

    addFocusManagement() {
        // Ensure file links are focusable
        const fileLinks = document.querySelectorAll('.file-link');
        fileLinks.forEach(link => {
            if (!link.hasAttribute('tabindex')) {
                link.setAttribute('tabindex', '0');
            }
        });
    }

    addSkipLinks() {
        // Add skip to content link for screen readers
        const skipLink = document.createElement('a');
        skipLink.href = '#main-content';
        skipLink.textContent = 'Skip to main content';
        skipLink.className = 'skip-link';
        skipLink.style.cssText = `
            position: absolute;
            top: -40px;
            left: 6px;
            background: var(--primary-color);
            color: white;
            padding: 8px;
            text-decoration: none;
            border-radius: 4px;
            z-index: 1000;
        `;
        
        skipLink.addEventListener('focus', () => {
            skipLink.style.top = '6px';
        });
        
        skipLink.addEventListener('blur', () => {
            skipLink.style.top = '-40px';
        });
        
        document.body.insertBefore(skipLink, document.body.firstChild);
        
        // Add id to main content
        const mainContent = document.querySelector('.main-content');
        if (mainContent) {
            mainContent.id = 'main-content';
        }
    }
}

// Performance optimizations
class PerformanceManager {
    constructor() {
        this.init();
    }

    init() {
        this.addLazyLoading();
        this.optimizeAnimations();
    }

    addLazyLoading() {
        // Intersection Observer for file items (useful for large directories)
        if ('IntersectionObserver' in window) {
            const observer = new IntersectionObserver((entries) => {
                entries.forEach(entry => {
                    if (entry.isIntersecting) {
                        entry.target.classList.add('visible');
                        observer.unobserve(entry.target);
                    }
                });
            }, { threshold: 0.1 });

            const fileItems = document.querySelectorAll('.file-item');
            fileItems.forEach(item => observer.observe(item));
        }
    }

    optimizeAnimations() {
        // Reduce animations for users who prefer reduced motion
        if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
            document.documentElement.style.setProperty('--transition-duration', '0s');
        }
    }
}

// Initialize everything when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new ThemeManager();
    new FileBrowser();
    new AccessibilityManager();
    new PerformanceManager();
    
    // Add a subtle loading animation
    document.body.style.opacity = '0';
    document.body.style.transition = 'opacity 0.3s ease';
    
    requestAnimationFrame(() => {
        document.body.style.opacity = '1';
    });
});

// Service Worker registration for offline support (optional)
if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
        // Uncomment to enable service worker
        // navigator.serviceWorker.register('/sw.js');
    });
}
