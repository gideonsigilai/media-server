// Video Player functionality
class MediaPlayer {
    constructor() {
        this.player = document.getElementById('main-player');
        this.container = document.getElementById('video-container');
        this.overlay = document.getElementById('video-overlay');
        this.fullscreenBtn = document.getElementById('fullscreen-btn');
        this.playlist = document.querySelectorAll('.playlist-item');

        this.init();
    }

    init() {
        this.setupEventListeners();
        this.setupPlaylist();
        this.setupKeyboardControls();
        this.setupFullscreen();
        this.setupAutoplay();
    }

    setupEventListeners() {
        if (this.player) {
            // Auto-hide info overlay after 3 seconds of no mouse movement
            let hideTimeout;
            const infoOverlay = document.getElementById('media-info-overlay');
            if (infoOverlay) {
                this.container.addEventListener('mousemove', () => {
                    infoOverlay.style.opacity = '1';
                    clearTimeout(hideTimeout);
                    hideTimeout = setTimeout(() => {
                        if (!this.player.paused) {
                            infoOverlay.style.opacity = '0';
                        }
                    }, 3000);
                });

                // Show info overlay when paused
                this.player.addEventListener('pause', () => {
                    infoOverlay.style.opacity = '1';
                });

                // Hide info overlay when playing
                this.player.addEventListener('play', () => {
                    setTimeout(() => {
                        infoOverlay.style.opacity = '0';
                    }, 1000);
                });
            }

            // Handle video errors
            this.player.addEventListener('error', (e) => {
                console.error('Media playback error:', e);
                console.error('Error details:', this.player.error);
                this.showError(`Unable to play this media file. Error: ${this.player.error ? this.player.error.message : 'Unknown error'}`);
            });

            // Handle video loading
            this.player.addEventListener('loadstart', () => {
                console.log('Media loading started');
                this.showLoading();
            });

            this.player.addEventListener('canplay', () => {
                console.log('Media can start playing');
                this.hideLoading();
            });

            // Add more detailed event logging
            this.player.addEventListener('loadedmetadata', () => {
                console.log('Media metadata loaded');
            });

            this.player.addEventListener('loadeddata', () => {
                console.log('Media data loaded');
            });

            this.player.addEventListener('stalled', () => {
                console.warn('Media loading stalled');
            });

            this.player.addEventListener('waiting', () => {
                console.log('Media waiting for data');
            });
        }
    }

    setupPlaylist() {
        this.playlist.forEach(item => {
            item.addEventListener('click', (e) => {
                e.preventDefault();
                this.playItem(item);
            });
        });
    }

    playItem(item) {
        const src = item.dataset.src;
        const title = item.dataset.title;
        const playerUrl = item.dataset.playerUrl;

        // Update active state
        this.playlist.forEach(p => p.classList.remove('active'));
        item.classList.add('active');

        // Navigate to the new player URL
        window.location.href = playerUrl;
    }

    setupAutoplay() {
        if (this.player) {
            // Try to autoplay when the page loads
            this.player.addEventListener('loadeddata', () => {
                // Attempt autoplay with user gesture fallback
                const playPromise = this.player.play();

                if (playPromise !== undefined) {
                    playPromise.then(() => {
                        console.log('Autoplay started successfully');
                    }).catch(error => {
                        console.log('Autoplay prevented by browser:', error);
                        // Show a play button or message to user
                        this.showPlayButton();
                    });
                }
            });
        }
    }

    showPlayButton() {
        // Create a play button overlay for when autoplay is blocked
        const playButton = document.createElement('div');
        playButton.className = 'play-button-overlay';
        playButton.innerHTML = `
            <div class="play-button">
                <span class="play-icon">▶️</span>
                <span class="play-text">Click to Play</span>
            </div>
        `;

        playButton.addEventListener('click', () => {
            this.player.play();
            playButton.remove();
        });

        this.container.appendChild(playButton);
    }

    setupKeyboardControls() {
        document.addEventListener('keydown', (e) => {
            if (!this.player) return;

            switch (e.code) {
                case 'Space':
                    e.preventDefault();
                    this.togglePlayPause();
                    break;
                case 'ArrowLeft':
                    e.preventDefault();
                    this.seekBackward();
                    break;
                case 'ArrowRight':
                    e.preventDefault();
                    this.seekForward();
                    break;
                case 'ArrowUp':
                    e.preventDefault();
                    this.volumeUp();
                    break;
                case 'ArrowDown':
                    e.preventDefault();
                    this.volumeDown();
                    break;
                case 'KeyF':
                    e.preventDefault();
                    this.toggleFullscreen();
                    break;
                case 'KeyM':
                    e.preventDefault();
                    this.toggleMute();
                    break;
                case 'Escape':
                    if (document.fullscreenElement) {
                        this.exitFullscreen();
                    }
                    break;
            }
        });
    }

    setupFullscreen() {
        if (this.fullscreenBtn) {
            this.fullscreenBtn.addEventListener('click', () => {
                this.toggleFullscreen();
            });
        }

        // Listen for fullscreen changes
        document.addEventListener('fullscreenchange', () => {
            const playerContainer = document.querySelector('.player-container');
            if (document.fullscreenElement) {
                playerContainer.classList.add('fullscreen');
            } else {
                playerContainer.classList.remove('fullscreen');
            }
        });
    }

    togglePlayPause() {
        if (this.player.paused) {
            this.player.play();
        } else {
            this.player.pause();
        }
    }

    seekBackward() {
        this.player.currentTime = Math.max(0, this.player.currentTime - 10);
    }

    seekForward() {
        this.player.currentTime = Math.min(this.player.duration, this.player.currentTime + 10);
    }

    volumeUp() {
        this.player.volume = Math.min(1, this.player.volume + 0.1);
    }

    volumeDown() {
        this.player.volume = Math.max(0, this.player.volume - 0.1);
    }

    toggleMute() {
        this.player.muted = !this.player.muted;
    }

    toggleFullscreen() {
        if (!document.fullscreenElement) {
            this.container.requestFullscreen().catch(err => {
                console.error('Error attempting to enable fullscreen:', err);
            });
        } else {
            this.exitFullscreen();
        }
    }

    exitFullscreen() {
        if (document.fullscreenElement) {
            document.exitFullscreen();
        }
    }

    showError(message) {
        const errorDiv = document.createElement('div');
        errorDiv.className = 'player-error';
        errorDiv.innerHTML = `
            <div class="error-icon">⚠️</div>
            <div class="error-message">${message}</div>
        `;
        this.container.appendChild(errorDiv);
    }

    showLoading() {
        const loadingDiv = document.createElement('div');
        loadingDiv.className = 'player-loading';
        loadingDiv.innerHTML = `
            <div class="loading-spinner">⏳</div>
            <div class="loading-message">Loading...</div>
        `;
        this.container.appendChild(loadingDiv);
    }

    hideLoading() {
        const loading = this.container.querySelector('.player-loading');
        if (loading) {
            loading.remove();
        }
    }
}

// Playlist navigation
class PlaylistManager {
    constructor() {
        this.currentIndex = 0;
        this.playlist = Array.from(document.querySelectorAll('.playlist-item'));
        this.init();
    }

    init() {
        this.findCurrentIndex();
        this.setupAutoplay();
    }

    findCurrentIndex() {
        this.playlist.forEach((item, index) => {
            if (item.classList.contains('active')) {
                this.currentIndex = index;
            }
        });
    }

    setupAutoplay() {
        const player = document.getElementById('main-player');
        if (player && player.tagName === 'VIDEO') {
            player.addEventListener('ended', () => {
                this.playNext();
            });
        }
    }

    playNext() {
        if (this.currentIndex < this.playlist.length - 1) {
            const nextItem = this.playlist[this.currentIndex + 1];
            const playerUrl = nextItem.dataset.playerUrl;
            window.location.href = playerUrl;
        }
    }

    playPrevious() {
        if (this.currentIndex > 0) {
            const prevItem = this.playlist[this.currentIndex - 1];
            const playerUrl = prevItem.dataset.playerUrl;
            window.location.href = playerUrl;
        }
    }
}

// Initialize when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new MediaPlayer();
    new PlaylistManager();

    // Add keyboard shortcuts info
    const shortcutsInfo = document.createElement('div');
    shortcutsInfo.className = 'shortcuts-info';
    shortcutsInfo.innerHTML = `
        <div class="shortcuts-title">Keyboard Shortcuts:</div>
        <div class="shortcuts-list">
            <span>Space: Play/Pause</span>
            <span>← →: Seek ±10s</span>
            <span>↑ ↓: Volume</span>
            <span>F: Fullscreen</span>
            <span>M: Mute</span>
        </div>
    `;

    // Show shortcuts on hover over player
    const playerContainer = document.querySelector('.player-container');
    if (playerContainer) {
        playerContainer.appendChild(shortcutsInfo);
    }
});
