// Library functionality
class MediaLibrary {
    constructor() {
        this.searchInput = document.getElementById('search-input');
        this.tabButtons = document.querySelectorAll('.tab-btn');
        this.tabContents = document.querySelectorAll('.tab-content');
        this.viewButtons = document.querySelectorAll('.view-btn');
        this.mediaGrids = document.querySelectorAll('.media-grid');
        this.mediaCards = document.querySelectorAll('.media-card');
        
        this.currentTab = 'videos';
        this.currentView = 'grid';
        
        this.init();
    }

    init() {
        this.setupTabs();
        this.setupSearch();
        this.setupViewToggle();
        this.setupCardInteractions();
        this.setupKeyboardNavigation();
    }

    setupTabs() {
        this.tabButtons.forEach(button => {
            button.addEventListener('click', () => {
                const tabId = button.dataset.tab;
                this.switchTab(tabId);
            });
        });
    }

    switchTab(tabId) {
        // Update tab buttons
        this.tabButtons.forEach(btn => {
            btn.classList.toggle('active', btn.dataset.tab === tabId);
        });

        // Update tab contents
        this.tabContents.forEach(content => {
            content.classList.toggle('active', content.id === `${tabId}-tab`);
        });

        this.currentTab = tabId;
        this.updateViewButtons();
    }

    setupSearch() {
        if (this.searchInput) {
            this.searchInput.addEventListener('input', (e) => {
                this.filterMedia(e.target.value);
            });

            // Clear search on escape
            this.searchInput.addEventListener('keydown', (e) => {
                if (e.key === 'Escape') {
                    this.searchInput.value = '';
                    this.filterMedia('');
                }
            });
        }
    }

    filterMedia(searchTerm) {
        const activeGrid = document.querySelector(`#${this.currentTab}-grid`);
        const cards = activeGrid.querySelectorAll('.media-card');
        
        cards.forEach(card => {
            const title = card.dataset.title.toLowerCase();
            const matches = title.includes(searchTerm.toLowerCase());
            card.style.display = matches ? 'block' : 'none';
        });

        // Update count
        const visibleCards = activeGrid.querySelectorAll('.media-card[style*="block"], .media-card:not([style*="none"])');
        this.updateResultsCount(visibleCards.length);
    }

    updateResultsCount(count) {
        const activeTab = document.querySelector('.tab-btn.active');
        const originalText = activeTab.textContent.split('(')[0];
        activeTab.innerHTML = `
            <span class="tab-icon">${activeTab.querySelector('.tab-icon').textContent}</span>
            ${originalText}(${count})
        `;
    }

    setupViewToggle() {
        this.viewButtons.forEach(button => {
            button.addEventListener('click', () => {
                const view = button.dataset.view;
                this.switchView(view);
            });
        });
    }

    switchView(view) {
        this.currentView = view;
        
        // Update view buttons in active section
        const activeSection = document.querySelector('.tab-content.active');
        const sectionViewButtons = activeSection.querySelectorAll('.view-btn');
        
        sectionViewButtons.forEach(btn => {
            btn.classList.toggle('active', btn.dataset.view === view);
        });

        // Update grid layout
        const activeGrid = activeSection.querySelector('.media-grid');
        activeGrid.classList.toggle('list-view', view === 'list');
    }

    updateViewButtons() {
        // Reset view buttons for the current tab
        const activeSection = document.querySelector('.tab-content.active');
        const sectionViewButtons = activeSection.querySelectorAll('.view-btn');
        
        sectionViewButtons.forEach(btn => {
            btn.classList.toggle('active', btn.dataset.view === this.currentView);
        });

        // Apply current view to the grid
        const activeGrid = activeSection.querySelector('.media-grid');
        activeGrid.classList.toggle('list-view', this.currentView === 'list');
    }

    setupCardInteractions() {
        this.mediaCards.forEach(card => {
            // Add hover effects
            card.addEventListener('mouseenter', () => {
                this.preloadThumbnail(card);
            });

            // Add click analytics (optional)
            card.addEventListener('click', () => {
                this.trackMediaClick(card);
            });
        });
    }

    preloadThumbnail(card) {
        const img = card.querySelector('.thumbnail-image');
        if (img && !img.complete) {
            // Add loading state
            card.classList.add('loading');
            
            img.addEventListener('load', () => {
                card.classList.remove('loading');
            }, { once: true });
        }
    }

    trackMediaClick(card) {
        const mediaType = card.dataset.type;
        const mediaTitle = card.dataset.title;
        
        // Log for analytics (could be sent to server)
        console.log(`Media clicked: ${mediaType} - ${mediaTitle}`);
    }

    setupKeyboardNavigation() {
        document.addEventListener('keydown', (e) => {
            switch (e.key) {
                case '1':
                    if (e.ctrlKey || e.metaKey) {
                        e.preventDefault();
                        this.switchTab('videos');
                    }
                    break;
                case '2':
                    if (e.ctrlKey || e.metaKey) {
                        e.preventDefault();
                        this.switchTab('audios');
                    }
                    break;
                case '3':
                    if (e.ctrlKey || e.metaKey) {
                        e.preventDefault();
                        this.switchTab('images');
                    }
                    break;
                case '/':
                    e.preventDefault();
                    this.searchInput.focus();
                    break;
                case 'g':
                    if (e.ctrlKey || e.metaKey) {
                        e.preventDefault();
                        this.switchView('grid');
                    }
                    break;
                case 'l':
                    if (e.ctrlKey || e.metaKey) {
                        e.preventDefault();
                        this.switchView('list');
                    }
                    break;
            }
        });
    }
}

// Infinite scroll for large libraries
class InfiniteScroll {
    constructor() {
        this.loading = false;
        this.page = 1;
        this.hasMore = true;
        
        this.init();
    }

    init() {
        window.addEventListener('scroll', () => {
            if (this.shouldLoadMore()) {
                this.loadMore();
            }
        });
    }

    shouldLoadMore() {
        const scrollTop = window.pageYOffset;
        const windowHeight = window.innerHeight;
        const documentHeight = document.documentElement.scrollHeight;
        
        return !this.loading && this.hasMore && (scrollTop + windowHeight >= documentHeight - 1000);
    }

    async loadMore() {
        if (this.loading) return;
        
        this.loading = true;
        this.showLoadingIndicator();
        
        try {
            // In a real implementation, this would fetch more data from the server
            await this.simulateLoading();
            this.page++;
        } catch (error) {
            console.error('Error loading more content:', error);
        } finally {
            this.loading = false;
            this.hideLoadingIndicator();
        }
    }

    simulateLoading() {
        return new Promise(resolve => setTimeout(resolve, 1000));
    }

    showLoadingIndicator() {
        const indicator = document.createElement('div');
        indicator.className = 'loading-indicator';
        indicator.innerHTML = `
            <div class="loading-spinner">⏳</div>
            <div class="loading-text">Loading more content...</div>
        `;
        document.body.appendChild(indicator);
    }

    hideLoadingIndicator() {
        const indicator = document.querySelector('.loading-indicator');
        if (indicator) {
            indicator.remove();
        }
    }
}

// Initialize when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new MediaLibrary();
    new InfiniteScroll();
    
    // Add keyboard shortcuts info
    const shortcutsInfo = document.createElement('div');
    shortcutsInfo.className = 'library-shortcuts';
    shortcutsInfo.innerHTML = `
        <div class="shortcuts-toggle">?</div>
        <div class="shortcuts-panel">
            <h4>Keyboard Shortcuts</h4>
            <div class="shortcuts-grid">
                <span>Ctrl+1/2/3:</span><span>Switch tabs</span>
                <span>/:</span><span>Focus search</span>
                <span>Ctrl+G:</span><span>Grid view</span>
                <span>Ctrl+L:</span><span>List view</span>
                <span>Esc:</span><span>Clear search</span>
            </div>
        </div>
    `;
    
    document.body.appendChild(shortcutsInfo);
    
    // Toggle shortcuts panel
    const shortcutsToggle = shortcutsInfo.querySelector('.shortcuts-toggle');
    const shortcutsPanel = shortcutsInfo.querySelector('.shortcuts-panel');
    
    shortcutsToggle.addEventListener('click', () => {
        shortcutsPanel.classList.toggle('visible');
    });
});
