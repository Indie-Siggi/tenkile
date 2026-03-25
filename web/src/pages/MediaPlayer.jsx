import { useState, useEffect, useRef } from 'preact/hooks';
import { route } from 'preact-router';
import Hls from 'hls.js';
import { fetchMediaItem, fetchHLSManifest } from '../hooks/useApi.js';

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const ALNUM_RE = /^[a-zA-Z0-9_-]+$/;

function isValidId(value) {
  return typeof value === 'string' && (UUID_RE.test(value) || ALNUM_RE.test(value));
}

export default function MediaPlayer({ id }) {
  const videoRef = useRef(null);
  const hlsRef = useRef(null);
  const safariListenerRef = useRef(null);
  
  const [mediaItem, setMediaItem] = useState(null);
  const [manifest, setManifest] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [selectedVariant, setSelectedVariant] = useState('1080p');
  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(1);
  const [muted, setMuted] = useState(false);

  useEffect(() => {
    if (id) {
      loadMedia();
    }
    return () => {
      destroyHls();
    };
  }, [id]);

  const loadMedia = async () => {
    try {
      setLoading(true);
      
      // Load media item info
      const item = await fetchMediaItem(id);
      setMediaItem(item);

      // Get HLS manifest
      const manifestData = await fetchHLSManifest(id, selectedVariant);
      setManifest(manifestData);

      // Initialize HLS player
      if (manifestData?.manifest) {
        initializeHls(manifestData.manifest);
      } else {
        setError('No playable stream found for this media item.');
        setLoading(false);
        return;
      }

      setError(null);
    } catch (err) {
      console.error('Failed to load media:', err);
      setError('Unable to load media. Please try again later.');
    } finally {
      setLoading(false);
    }
  };

  const initializeHls = (manifestUrl) => {
    destroyHls();

    const video = videoRef.current;
    if (!video) return;

    // Normalize manifest URL to absolute
    let fullUrl = manifestUrl;
    if (manifestUrl.startsWith('/')) {
      fullUrl = `${window.location.origin}${manifestUrl}`;
    } else if (!manifestUrl.startsWith('http')) {
      // For file paths from manifest, proxy through our API
      fullUrl = `/api/v1/stream/hls/playlist?path=${encodeURIComponent(manifestUrl)}`;
    }

    // Validate URL is same-origin to prevent loading from untrusted sources
    try {
      const parsed = new URL(fullUrl, window.location.origin);
      if (parsed.origin !== window.location.origin) {
        setError('Cannot load media from an external source.');
        return;
      }
    } catch {
      setError('Invalid media URL.');
      return;
    }

    if (Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: true,
        lowLatencyMode: false,
      });

      hls.loadSource(fullUrl);
      hls.attachMedia(video);

      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        video.play().catch(console.error);
      });

      hls.on(Hls.Events.ERROR, (event, data) => {
        if (data.fatal) {
          console.error('HLS fatal error:', data);
          switch (data.type) {
            case Hls.ErrorTypes.NETWORK_ERROR:
              hls.startLoad();
              break;
            case Hls.ErrorTypes.MEDIA_ERROR:
              hls.recoverMediaError();
              break;
            default:
              destroyHls();
              break;
          }
        }
      });

      hlsRef.current = hls;
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      // Native HLS support (Safari)
      video.src = fullUrl;
      const onLoadedMetadata = () => {
        video.play().catch(console.error);
      };
      safariListenerRef.current = onLoadedMetadata;
      video.addEventListener('loadedmetadata', onLoadedMetadata);
    } else {
      setError('HLS is not supported in this browser');
    }
  };

  const destroyHls = () => {
    if (hlsRef.current) {
      hlsRef.current.destroy();
      hlsRef.current = null;
    }
    // Clean up Safari native HLS listener
    if (safariListenerRef.current && videoRef.current) {
      videoRef.current.removeEventListener('loadedmetadata', safariListenerRef.current);
      safariListenerRef.current = null;
    }
  };

  const handleVariantChange = (variant) => {
    setSelectedVariant(variant);
    if (manifest?.manifest) {
      // Reload with new variant
      initializeHls(manifest.manifest);
    }
  };

  const handlePlay = () => {
    const video = videoRef.current;
    if (video) {
      if (video.paused) {
        video.play();
      } else {
        video.pause();
      }
    }
  };

  const handleSeek = (e) => {
    const video = videoRef.current;
    if (video) {
      video.currentTime = parseFloat(e.target.value);
    }
  };

  const handleVolumeChange = (e) => {
    const video = videoRef.current;
    const newVolume = parseFloat(e.target.value);
    if (video) {
      video.volume = newVolume;
      setVolume(newVolume);
      setMuted(newVolume === 0);
    }
  };

  const toggleMute = () => {
    const video = videoRef.current;
    if (video) {
      video.muted = !video.muted;
      setMuted(video.muted);
    }
  };

  const handleBack = () => {
    destroyHls();
    const libraryId = mediaItem?.library_id;
    if (libraryId && isValidId(libraryId)) {
      route(`/library/${libraryId}`);
    } else {
      route('/');
    }
  };

  const formatTime = (seconds) => {
    if (isNaN(seconds)) return '0:00';
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    if (h > 0) {
      return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
    }
    return `${m}:${s.toString().padStart(2, '0')}`;
  };

  const handleTimeUpdate = () => {
    const video = videoRef.current;
    if (video) {
      setCurrentTime(video.currentTime);
    }
  };

  const handleLoadedMetadata = () => {
    const video = videoRef.current;
    if (video) {
      setDuration(video.duration);
    }
  };

  const handlePlayState = () => {
    const video = videoRef.current;
    if (video) {
      setPlaying(!video.paused);
    }
  };

  if (loading) {
    return (
      <div class="media-player loading">
        <div class="spinner"></div>
        <p>Loading media...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div class="media-player error">
        <div class="error-card">
          <h2>Error</h2>
          <p>{error}</p>
          <button class="btn btn-primary" onClick={loadMedia}>
            Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div class="media-player">
      <button class="btn btn-back" onClick={handleBack}>
        &larr; Back to Library
      </button>

      <div class="player-container">
        <div class="video-wrapper">
          <video
            ref={videoRef}
            onTimeUpdate={handleTimeUpdate}
            onLoadedMetadata={handleLoadedMetadata}
            onPlay={() => setPlaying(true)}
            onPause={() => setPlaying(false)}
            onClick={handlePlay}
          />
          
          <div class="play-overlay" onClick={handlePlay}>
            {!playing && <span class="big-play-icon">▶</span>}
          </div>
        </div>

        <div class="player-controls">
          <div class="progress-bar">
            <input
              type="range"
              min="0"
              max={duration || 100}
              value={currentTime}
              onInput={handleSeek}
              class="seek-slider"
            />
            <div
              class="progress-fill"
              style={{ width: `${(currentTime / duration) * 100}%` }}
            />
          </div>

          <div class="controls-row">
            <div class="controls-left">
              <button class="control-btn" onClick={handlePlay}>
                {playing ? '⏸' : '▶'}
              </button>
              
              <div class="volume-control">
                <button class="control-btn" onClick={toggleMute}>
                  {muted || volume === 0 ? '🔇' : volume < 0.5 ? '🔉' : '🔊'}
                </button>
                <input
                  type="range"
                  min="0"
                  max="1"
                  step="0.1"
                  value={muted ? 0 : volume}
                  onInput={handleVolumeChange}
                  class="volume-slider"
                />
              </div>

              <span class="time-display">
                {formatTime(currentTime)} / {formatTime(duration)}
              </span>
            </div>

            <div class="controls-right">
              <select
                class="variant-select"
                value={selectedVariant}
                onChange={(e) => handleVariantChange(e.target.value)}
              >
                <option value="4k">4K</option>
                <option value="1080p">1080p</option>
                <option value="720p">720p</option>
                <option value="480p">480p</option>
                <option value="360p">360p</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      <div class="media-info">
        <h1>{mediaItem?.title || 'Unknown Title'}</h1>
        {mediaItem?.year && <span class="year">{mediaItem.year}</span>}
        {mediaItem?.overview && <p class="overview">{mediaItem.overview}</p>}
      </div>
    </div>
  );
}
