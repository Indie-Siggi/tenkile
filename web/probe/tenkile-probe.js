// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

/**
 * Tenkile Probe - Client-side device capability detection
 *
 * This module provides device probing functionality to detect
 * codec support, playback capabilities, and streaming performance.
 */

/**
 * TenkileProbe class for device capability detection
 */
class TenkileProbe {
  /**
   * Create a new TenkileProbe instance
   * @param {Object} options - Probe configuration
   * @param {string} options.serverUrl - Tenkile server URL
   * @param {string} options.deviceId - Device identifier
   * @param {number} options.timeout - Timeout in milliseconds for probe tests
   * @param {number} options.maxRetries - Maximum retry attempts
   */
  constructor(options = {}) {
    this.serverUrl = options.serverUrl || window.location.origin;
    this.deviceId = options.deviceId || this.generateDeviceId();
    this.timeout = options.timeout || 10000;
    this.maxRetries = options.maxRetries || 3;
    
    this.results = {
      identity: this.getIdentity(),
      videoCodecs: [],
      audioCodecs: [],
      subtitleFormats: [],
      containerFormats: [],
      maxResolution: { width: 0, height: 0 },
      maxBitrate: 0,
      supportsHDR: false,
      supportsDV: false,
      supportsAtmos: false,
      supportsDTS: false,
      scenarioSupport: [],
      errors: [],
      warnings: [],
      timestamp: new Date().toISOString()
    };
  }

  /**
   * Generate a unique device identifier
   * @returns {string} Device ID
   */
  generateDeviceId() {
    if (window.crypto && window.crypto.randomUUID) {
      return window.crypto.randomUUID();
    }
    return 'probe-' + Date.now().toString(36) + '-' + Math.random().toString(36).substr(2, 9);
  }

  /**
   * Get device identity information
   * @returns {Object} Device identity
   */
  getIdentity() {
    const ua = navigator.userAgent;
    const platform = this.detectPlatform(ua);
    
    return {
      deviceId: this.deviceId,
      name: platform.name || 'Unknown Device',
      model: platform.model || 'Unknown Model',
      manufacturer: platform.manufacturer || 'Unknown',
      platform: {
        type: platform.type || 'unknown',
        name: platform.name || 'Unknown',
        os: platform.os || 'Unknown OS',
        osVersion: platform.osVersion || '',
        webBrowser: platform.browser || '',
        browserVer: platform.browserVersion || ''
      },
      userAgent: ua
    };
  }

  /**
   * Detect device platform from user agent
   * @param {string} ua - User agent string
   * @returns {Object} Platform information
   */
  detectPlatform(ua) {
    const result = { type: 'unknown', name: 'Unknown', os: 'Unknown', browser: '' };
    
    // Detect OS
    if (/Android/i.test(ua)) {
      result.os = 'Android';
      result.type = 'mobile';
    } else if (/iPhone|iPad|iPod/i.test(ua)) {
      result.os = 'iOS';
      result.type = /iPad/i.test(ua) ? 'tablet' : 'mobile';
    } else if (/Windows NT/i.test(ua)) {
      result.os = 'Windows';
      result.type = 'desktop';
    } else if (/Macintosh/i.test(ua)) {
      result.os = 'macOS';
      result.type = 'desktop';
    } else if (/Linux/i.test(ua)) {
      result.os = 'Linux';
      result.type = 'desktop';
    }
    
    // Detect browser
    if (/Chrome\/(\d+)/.test(ua)) {
      result.browser = 'Chrome';
      result.browserVersion = RegExp.$1;
    } else if (/Safari\/(\d+)/.test(ua) && !/Chrome/.test(ua)) {
      result.browser = 'Safari';
      result.browserVersion = RegExp.$1;
    } else if (/Firefox\/(\d+)/.test(ua)) {
      result.browser = 'Firefox';
      result.browserVersion = RegExp.$1;
    } else if (/Edge\/(\d+)/.test(ua)) {
      result.browser = 'Edge';
      result.browserVersion = RegExp.$1;
    }
    
    // Detect smart TV platforms
    if (/Roku|Dolby/i.test(ua)) {
      result.type = 'smart_tv';
      result.name = 'Roku';
    } else if (/AppleTV|Apple TV/i.test(ua)) {
      result.type = 'streaming_box';
      result.name = 'Apple TV';
    } else if (/Amazon|FireTV|Silk/i.test(ua)) {
      result.type = 'streaming_box';
      result.name = 'Fire TV';
    } else if (/CrKey|Chromecast/i.test(ua)) {
      result.type = 'streaming_box';
      result.name = 'Chromecast';
    }
    
    // Detect gaming consoles
    if (/PlayStation|PS4|PS5/i.test(ua)) {
      result.type = 'gaming_console';
      result.name = 'PlayStation';
    } else if (/Xbox/i.test(ua)) {
      result.type = 'gaming_console';
      result.name = 'Xbox';
    }
    
    return result;
  }

  /**
   * Detect supported video codecs
   * @returns {Promise<Array>} List of supported video codecs
   */
  async detectVideoCodecs() {
    const codecs = ['h264', 'hevc', 'vp9', 'av1', 'vp8'];
    const supported = [];
    
    for (const codec of codecs) {
      try {
        const result = await this.testVideoCodec(codec);
        if (result) {
          supported.push(codec);
        }
      } catch (err) {
        this.results.warnings.push(`Failed to test ${codec}: ${err.message}`);
      }
    }
    
    this.results.videoCodecs = supported;
    return supported;
  }

  /**
   * Test if a specific video codec is supported
   * @param {string} codec - Codec name
   * @returns {Promise<boolean>} True if supported
   */
  async testVideoCodec(codec) {
    // Use video element canPlayType for basic detection
    const video = document.createElement('video');
    
    const mimeTypes = {
      'h264': 'video/mp4; codecs="avc1.42E01E"',
      'hevc': 'video/mp4; codecs="hvc1"',
      'vp9': 'video/webm; codecs="vp9"',
      'av1': 'video/webm; codecs="av01.0.05M.08"',
      'vp8': 'video/webm; codecs="vp8"'
    };
    
    const mime = mimeTypes[codec];
    if (!mime) return false;
    
    const canPlay = video.canPlayType(mime);
    return canPlay === 'probably' || canPlay === 'maybe';
  }

  /**
   * Detect supported audio codecs
   * @returns {Promise<Array>} List of supported audio codecs
   */
  async detectAudioCodecs() {
    const codecs = ['aac', 'mp3', 'opus', 'flac', 'ac3', 'eac3', 'dts'];
    const supported = [];
    
    for (const codec of codecs) {
      try {
        const result = await this.testAudioCodec(codec);
        if (result) {
          supported.push(codec);
        }
      } catch (err) {
        this.results.warnings.push(`Failed to test ${codec}: ${err.message}`);
      }
    }
    
    this.results.audioCodecs = supported;
    return supported;
  }

  /**
   * Test if a specific audio codec is supported
   * @param {string} codec - Codec name
   * @returns {Promise<boolean>} True if supported
   */
  async testAudioCodec(codec) {
    const audio = document.createElement('audio');
    
    const mimeTypes = {
      'aac': 'audio/mp4; codecs="mp4a.40.2"',
      'mp3': 'audio/mpeg',
      'opus': 'audio/ogg; codecs="opus"',
      'flac': 'audio/flac',
      'ac3': 'audio/ac3',
      'eac3': 'audio/eac3',
      'dts': 'audio/dts'
    };
    
    const mime = mimeTypes[codec];
    if (!mime) return false;
    
    const canPlay = audio.canPlayType(mime);
    return canPlay === 'probably' || canPlay === 'maybe';
  }

  /**
   * Detect supported subtitle formats
   * @returns {Promise<Array>} List of supported subtitle formats
   */
  async detectSubtitleFormats() {
    const formats = ['srt', 'vtt', 'ass', 'ssa'];
    const supported = [];
    
    // VTT is widely supported in modern browsers
    if ('VTTCue' in window) {
      supported.push('vtt');
    }
    
    // SRT can be parsed with TextTracks
    if ('TextTrack' in window) {
      supported.push('srt');
    }
    
    this.results.subtitleFormats = supported;
    return supported;
  }

  /**
   * Detect maximum display resolution
   * @returns {Object} Resolution information
   */
  detectMaxResolution() {
    const width = window.screen.width;
    const height = window.screen.height;
    const pixelRatio = window.devicePixelRatio || 1;
    
    this.results.maxResolution = {
      width: Math.floor(width * pixelRatio),
      height: Math.floor(height * pixelRatio)
    };
    
    return this.results.maxResolution;
  }

  /**
   * Run a playback scenario test
   * @param {Object} scenario - Scenario configuration
   * @returns {Promise<Object>} Scenario result
   */
  async runScenario(scenario) {
    const startTime = Date.now();
    const result = {
      scenarioId: scenario.id,
      passed: false,
      directPlay: false,
      bitrate: scenario.bitrate || 0,
      durationMs: 0,
      errors: []
    };
    
    try {
      // Create test media element
      const media = scenario.type === 'audio' 
        ? document.createElement('audio')
        : document.createElement('video');
      
      media.src = scenario.url;
      media.crossOrigin = 'anonymous';
      
      // Set timeout
      const timeoutPromise = new Promise((_, reject) => {
        setTimeout(() => reject(new Error('Test timeout')), this.timeout);
      });
      
      // Wait for canplaythrough
      const playPromise = new Promise((resolve, reject) => {
        media.addEventListener('canplaythrough', () => resolve(true), { once: true });
        media.addEventListener('error', (e) => reject(new Error('Playback error')), { once: true });
      });
      
      await Promise.race([playPromise, timeoutPromise]);
      
      result.passed = true;
      result.directPlay = scenario.forceDirectPlay || false;
      result.durationMs = Date.now() - startTime;
      
      // Clean up
      media.src = '';
      
    } catch (err) {
      result.errors.push(err.message);
      this.results.errors.push(`Scenario ${scenario.id} failed: ${err.message}`);
    }
    
    return result;
  }

  /**
   * Run all probe tests
   * @returns {Promise<Object>} Complete probe results
   */
  async run() {
    console.log('Tenkile Probe: Starting device capability detection...');
    
    // Run detection tests
    await this.detectVideoCodecs();
    await this.detectAudioCodecs();
    await this.detectSubtitleFormats();
    this.detectMaxResolution();
    
    // Run scenario tests
    const scenarios = this.getScenarios();
    for (const scenario of scenarios) {
      const result = await this.runScenario(scenario);
      this.results.scenarioSupport.push(result);
    }
    
    // Calculate trust score
    this.results.trustLevel = this.calculateTrustLevel();
    this.results.trustScore = this.calculateTrustScore();
    
    console.log('Tenkile Probe: Detection complete', this.results);
    
    return this.results;
  }

  /**
   * Get default probe scenarios
   * @returns {Array} Scenario list
   */
  getScenarios() {
    return [
      { id: 'h264_1080p', type: 'video', bitrate: 5000000, forceDirectPlay: true },
      { id: 'h264_720p', type: 'video', bitrate: 2500000, forceDirectPlay: true },
      { id: 'aac_stereo', type: 'audio', bitrate: 192000, forceDirectPlay: true }
    ];
  }

  /**
   * Calculate trust level based on detection results
   * @returns {string} Trust level
   */
  calculateTrustLevel() {
    const supportedCount = this.results.videoCodecs.length + this.results.audioCodecs.length;
    
    if (supportedCount >= 10) return 'trusted';
    if (supportedCount >= 7) return 'high';
    if (supportedCount >= 4) return 'medium';
    if (supportedCount >= 2) return 'low';
    return 'unknown';
  }

  /**
   * Calculate trust score (0-1)
   * @returns {number} Trust score
   */
  calculateTrustScore() {
    const baseScore = (this.results.videoCodecs.length + this.results.audioCodecs.length) / 15;
    const resolutionBonus = this.results.maxResolution.width >= 3840 ? 0.1 : 0;
    const hdrBonus = this.results.supportsHDR ? 0.1 : 0;
    
    return Math.min(1.0, baseScore + resolutionBonus + hdrBonus);
  }

  /**
   * Report probe success to server
   * @param {Object} results - Probe results
   * @returns {Promise<Response>} Server response
   */
  async reportSuccess(results = this.results) {
    try {
      const response = await fetch(`${this.serverUrl}/api/v1/probe/report`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Device-ID': this.deviceId
        },
        body: JSON.stringify({
          success: true,
          data: results,
          timestamp: new Date().toISOString()
        })
      });
      
      console.log('Tenkile Probe: Success report sent', response.status);
      return response;
    } catch (err) {
      console.error('Tenkile Probe: Failed to report success', err);
      throw err;
    }
  }

  /**
   * Report probe failure to server
   * @param {string} error - Error message
   * @returns {Promise<Response>} Server response
   */
  async reportFailure(error) {
    try {
      const response = await fetch(`${this.serverUrl}/api/v1/probe/report`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Device-ID': this.deviceId
        },
        body: JSON.stringify({
          success: false,
          error: error,
          timestamp: new Date().toISOString()
        })
      });
      
      console.log('Tenkile Probe: Failure report sent', response.status);
      return response;
    } catch (err) {
      console.error('Tenkile Probe: Failed to report failure', err);
      throw err;
    }
  }
}

// Export for module systems
if (typeof module !== 'undefined' && module.exports) {
  module.exports = TenkileProbe;
}

// Make available globally
window.TenkileProbe = TenkileProbe;
