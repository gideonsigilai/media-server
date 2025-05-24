// Settings page functionality
class SettingsManager {
    constructor() {
        this.modal = document.getElementById('folder-modal');
        this.folderBrowser = document.getElementById('folder-browser');
        this.currentPathDisplay = document.getElementById('current-path-display');
        this.mediaDirectoryInput = document.getElementById('media_dir');
        this.currentPath = 'C:\\';
        
        this.init();
    }

    init() {
        this.setupFolderBrowser();
        this.setupUploadArea();
        this.setupDriveButtons();
        this.setupModal();
    }

    setupFolderBrowser() {
        const browseBtn = document.getElementById('browse-btn');
        if (browseBtn) {
            browseBtn.addEventListener('click', () => {
                this.openFolderBrowser();
            });
        }

        const selectFolderBtn = document.getElementById('select-folder');
        if (selectFolderBtn) {
            selectFolderBtn.addEventListener('click', () => {
                this.selectCurrentFolder();
            });
        }

        const cancelBtn = document.getElementById('cancel-select');
        if (cancelBtn) {
            cancelBtn.addEventListener('click', () => {
                this.closeFolderBrowser();
            });
        }
    }

    setupUploadArea() {
        const uploadArea = document.getElementById('upload-area');
        const fileInput = document.getElementById('file-input');
        const fileList = document.getElementById('file-list');
        const uploadBtn = document.getElementById('upload-btn');
        const clearBtn = document.getElementById('clear-btn');

        if (!uploadArea || !fileInput) return;

        // Drag and drop functionality
        uploadArea.addEventListener('dragover', (e) => {
            e.preventDefault();
            uploadArea.classList.add('dragover');
        });

        uploadArea.addEventListener('dragleave', (e) => {
            e.preventDefault();
            uploadArea.classList.remove('dragover');
        });

        uploadArea.addEventListener('drop', (e) => {
            e.preventDefault();
            uploadArea.classList.remove('dragover');
            
            const files = e.dataTransfer.files;
            this.handleFileSelection(files);
        });

        // File input change
        fileInput.addEventListener('change', (e) => {
            this.handleFileSelection(e.target.files);
        });

        // Clear button
        if (clearBtn) {
            clearBtn.addEventListener('click', () => {
                fileInput.value = '';
                fileList.innerHTML = '';
                uploadBtn.disabled = true;
            });
        }
    }

    setupDriveButtons() {
        const driveButtons = document.querySelectorAll('.drive-btn');
        driveButtons.forEach(btn => {
            btn.addEventListener('click', () => {
                const path = btn.dataset.path;
                this.mediaDirectoryInput.value = path;
            });
        });
    }

    setupModal() {
        const modalClose = document.getElementById('modal-close');
        if (modalClose) {
            modalClose.addEventListener('click', () => {
                this.closeFolderBrowser();
            });
        }

        // Close modal when clicking outside
        this.modal.addEventListener('click', (e) => {
            if (e.target === this.modal) {
                this.closeFolderBrowser();
            }
        });
    }

    handleFileSelection(files) {
        const fileList = document.getElementById('file-list');
        const uploadBtn = document.getElementById('upload-btn');
        
        if (files.length === 0) {
            uploadBtn.disabled = true;
            return;
        }

        fileList.innerHTML = '';
        
        Array.from(files).forEach(file => {
            const fileItem = document.createElement('div');
            fileItem.className = 'file-item';
            
            const fileName = document.createElement('span');
            fileName.textContent = file.name;
            
            const fileSize = document.createElement('span');
            fileSize.textContent = this.formatFileSize(file.size);
            fileSize.style.color = 'var(--text-muted)';
            
            fileItem.appendChild(fileName);
            fileItem.appendChild(fileSize);
            fileList.appendChild(fileItem);
        });

        uploadBtn.disabled = false;
    }

    formatFileSize(bytes) {
        const units = ['B', 'KB', 'MB', 'GB'];
        let size = bytes;
        let unitIndex = 0;
        
        while (size >= 1024 && unitIndex < units.length - 1) {
            size /= 1024;
            unitIndex++;
        }
        
        return `${size.toFixed(1)} ${units[unitIndex]}`;
    }

    openFolderBrowser() {
        this.currentPath = this.mediaDirectoryInput.value || 'C:\\';
        this.loadFolders(this.currentPath);
        this.modal.classList.add('show');
    }

    closeFolderBrowser() {
        this.modal.classList.remove('show');
    }

    selectCurrentFolder() {
        this.mediaDirectoryInput.value = this.currentPath;
        this.closeFolderBrowser();
    }

    async loadFolders(path) {
        try {
            this.currentPath = path;
            this.currentPathDisplay.textContent = path;
            
            const response = await fetch(`/browse-folder?path=${encodeURIComponent(path)}`);
            const html = await response.text();
            
            this.folderBrowser.innerHTML = html;
            
            // Add click handlers to folder items
            const folderItems = this.folderBrowser.querySelectorAll('.folder-item');
            folderItems.forEach(item => {
                item.addEventListener('click', () => {
                    const newPath = item.dataset.path;
                    this.loadFolders(newPath);
                });
            });
            
        } catch (error) {
            console.error('Error loading folders:', error);
            this.folderBrowser.innerHTML = '<div class="error">Error loading folders</div>';
        }
    }
}

// File upload with progress
class UploadManager {
    constructor() {
        this.setupProgressTracking();
    }

    setupProgressTracking() {
        const uploadForm = document.querySelector('.upload-form');
        if (!uploadForm) return;

        uploadForm.addEventListener('submit', (e) => {
            this.showUploadProgress();
        });
    }

    showUploadProgress() {
        const uploadBtn = document.getElementById('upload-btn');
        if (uploadBtn) {
            uploadBtn.disabled = true;
            uploadBtn.textContent = 'Uploading...';
            
            // Create progress bar
            const progressBar = document.createElement('div');
            progressBar.className = 'upload-progress';
            progressBar.innerHTML = `
                <div class="progress-bar">
                    <div class="progress-fill"></div>
                </div>
                <div class="progress-text">Uploading files...</div>
            `;
            
            uploadBtn.parentNode.appendChild(progressBar);
        }
    }
}

// Keyboard shortcuts for settings
class SettingsShortcuts {
    constructor() {
        this.setupKeyboardShortcuts();
    }

    setupKeyboardShortcuts() {
        document.addEventListener('keydown', (e) => {
            // Ctrl+U for upload
            if ((e.ctrlKey || e.metaKey) && e.key === 'u') {
                e.preventDefault();
                const fileInput = document.getElementById('file-input');
                if (fileInput) {
                    fileInput.click();
                }
            }
            
            // Ctrl+B for browse folder
            if ((e.ctrlKey || e.metaKey) && e.key === 'b') {
                e.preventDefault();
                const browseBtn = document.getElementById('browse-btn');
                if (browseBtn) {
                    browseBtn.click();
                }
            }
            
            // Escape to close modal
            if (e.key === 'Escape') {
                const modal = document.getElementById('folder-modal');
                if (modal && modal.classList.contains('show')) {
                    modal.classList.remove('show');
                }
            }
        });
    }
}

// Initialize when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new SettingsManager();
    new UploadManager();
    new SettingsShortcuts();
    
    // Handle URL parameters (like success messages)
    const urlParams = new URLSearchParams(window.location.search);
    const message = urlParams.get('message');
    if (message) {
        const alertDiv = document.createElement('div');
        alertDiv.className = 'alert alert-success';
        alertDiv.innerHTML = `<h3>✅ Success!</h3><p>${message}</p>`;
        
        const settingsContent = document.querySelector('.settings-content');
        if (settingsContent) {
            settingsContent.insertBefore(alertDiv, settingsContent.firstChild);
        }
        
        // Remove message from URL
        window.history.replaceState({}, document.title, window.location.pathname);
    }
});
