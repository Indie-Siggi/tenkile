/**
 * Tenkile Client-Side Probe Library
 *
 * Detects device capabilities and reports them to the Tenkile server.
 * Uses navigator.mediaCapabilities and HTML5 canPlayType for detection.
 *
 * @module tenkile-probe
 * @version 1.0.0
 * @license AGPL-3.0-or-later
 * @copyright (c) 2024 Tenkile Contributors
 */

(function (global) {
	'use strict';

	/**
	 * TenkileProbe - Client-side device capability detection
	 * @class
	 */
	class TenkileProbe {
		/**
		 * Creates a new TenkileProbe instance
		 * @param {Object} options - Configuration options
		 * @param {string} [options.serverUrl] - Base URL for the Tenkile server
		 * @param {string} [options.deviceId] - Custom device ID (generated if not provided)
		 * @param {number} [options.timeout=10000] - Request timeout in milliseconds
		 * @param {number} [options.maxRetries=3] - Maximum retry attempts
		 * @param {boolean} [options.verbose=false] - Enable verbose logging
		 */
		constructor(options = {}) {
			this.serverUrl = options.serverUrl || '';
			this.deviceId = options.deviceId || this._generateDeviceId();
			this.timeout = options.timeout || 10000;
			this.maxRetries = options.maxRetries || 3;
			this.verbose = options.verbose || false;

			// Playback feedback hooks
			this.reportSuccess = options.reportSuccess || null;
			this.reportFailure = options.reportFailure || null;

			// Progress callbacks
			this.onProgress = options.onProgress || null;
			this.onComplete = options.onComplete || null;
			this.onError = options.onError || null;

			// Internal state
			this._isProbing = false;
			this._probeStartTime = null;
			this._detectedCapabilities = null;
		}

		/**
		 * Generates a unique device ID
		 * @private
		 * @returns {string} Unique device identifier
		 */
		_generateDeviceId() {
			if (window.crypto && window.crypto.getRandomValues) {
				const array = new Uint8Array(16);
				window.crypto.getRandomValues(array);
				return Array.from(array, b => b.toString(16).padStart(2, '0')).join('');
			}
			return 'device-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
		}

		/**
		 * Logs a message if verbose mode is enabled
		 * @private
		 * @param {string} level - Log level
		 * @param {string} message - Message to log
		 */
		_log(level, message) {
			if (this.verbose) {
				console.log(`[TenkileProbe] [${level}] ${message}`);
			}
		}

		/**
		 * Reports progress to callbacks
		 * @private
		 * @param {number} progress - Progress percentage (0-100)
		 * @param {string} stage - Current probe stage
		 * @param {Object} [details] - Additional details
		 */
		_reportProgress(progress, stage, details = {}) {
			if (this.onProgress && typeof this.onProgress === 'function') {
				this.onProgress({
					progress: Math.min(100, Math.max(0, progress)),
					stage: stage,
					details: details,
					elapsedMs: this._probeStartTime ? Date.now() - this._probeStartTime : 0
				});
			}
		}

		/**
		 * Detects the browser platform
		 * @private
		 * @returns {string} Platform identifier
		 */
		_detectPlatform() {
			const userAgent = navigator.userAgent || navigator.vendor || window.opera;

			// iOS detection
			if (/iPad|iPhone|iPod/.test(userAgent) && !window.MSStream) {
				return 'ios';
			}

			// Android detection
			if (/Android/.test(userAgent)) {
				return 'android';
			}

			// Windows detection
			if (/Windows NT/.test(userAgent)) {
				return 'windows';
			}

			// macOS detection
			if (/Macintosh/.test(userAgent) || (/Mac OS X/.test(userAgent))) {
				return 'macos';
			}

			// Linux detection
			if (/Linux/.test(userAgent)) {
				return 'linux';
			}

			// Chrome OS detection
			if (/CrOS/.test(userAgent)) {
				return 'chromeos';
			}

			// Fire TV detection
			if (/Fire TV|FireTV|AFTR/.test(userAgent)) {
				return 'firetv';
			}

			// Roku detection
			if (/Roku/.test(userAgent)) {
				return 'roku';
			}

			// Chromecast detection
			if (/CrKey|Chromecast/.test(userAgent)) {
				return 'chromecast';
			}

			// Xbox detection
			if (/Xbox/.test(userAgent)) {
				return 'xbox';
			}

			// PlayStation detection
			if (/PlayStation/.test(userAgent)) {
				return 'playstation';
			}

			// Apple TV detection (often appears as Safari on AppleTV)
			if (/AppleTV/.test(userAgent)) {
				return 'tvos';
			}

			return 'web';
		}

		/**
		 * Detects browser/engine information
		 * @private
		 * @returns {Object} Browser information
		 */
		_detectBrowser() {
			const userAgent = navigator.userAgent;
			const vendor = navigator.vendor || '';

			const browser = {
				name: 'Unknown',
				version: 'Unknown',
				engine: 'Unknown',
				engineVersion: 'Unknown'
			};

			// Chrome detection
			if (/Chrom(e|ium)/.test(userAgent)) {
				browser.name = 'Chrome';
				const match = userAgent.match(/Chrom(?:e|ium)\/([0-9.]+)/);
				if (match) browser.version = match[1];
				browser.engine = 'Blink';
			}
			// Firefox detection
			else if (/Firefox/.test(userAgent)) {
				browser.name = 'Firefox';
				const match = userAgent.match(/Firefox\/([0-9.]+)/);
				if (match) browser.version = match[1];
				browser.engine = 'Gecko';
			}
			// Safari detection
			else if (/Safari/.test(userAgent) && !/Chrome/.test(userAgent)) {
				browser.name = 'Safari';
				const match = userAgent.match(/Version\/([0-9.]+)/);
				if (match) browser.version = match[1];
				browser.engine = 'WebKit';
			}
			// Edge detection
			else if (/Edg/.test(userAgent)) {
				browser.name = 'Edge';
				const match = userAgent.match(/Edg(?:e|A|iOS)?\/([0-9.]+)/);
				if (match) browser.version = match[1];
				browser.engine = 'Blink';
			}
			// Opera detection
			else if (/OPR|Opera/.test(userAgent)) {
				browser.name = 'Opera';
				const match = userAgent.match(/OPR\/([0-9.]+)/);
				if (match) browser.version = match[1];
				browser.engine = 'Blink';
			}
			// IE detection
			else if (/MSIE|Trident/.test(userAgent)) {
				browser.name = 'Internet Explorer';
				const match = userAgent.match(/MSIE ([0-9.]+)/) || userAgent.match(/rv:([0-9.]+)/);
				if (match) browser.version = match[1];
				browser.engine = 'Trident';
			}

			// Detect engine version for WebKit/Blink
			if (browser.engine === 'WebKit' || browser.engine === 'Blink') {
				const match = userAgent.match(/WebKit\/([0-9.]+)/);
				if (match) browser.engineVersion = match[1];
			}

			return browser;
		}

		/**
		 * Detects video codec support using canPlayType and mediaCapabilities
		 * @private
		 * @returns {Promise<Array>} List of supported video codecs
		 */
		async _detectVideoCodecs() {
			const codecs = new Set();
			const testCodecs = [
				'h264', 'avc1.64001f', 'avc1.640028', 'avc1.4d401f',
				'hevc', 'hvc1.1.6.L153.B0', 'hev1.1.6.L153.B0',
				'vp9', 'vp9.0', 'vp09.00.10.08',
				'vp8', 'vp8.0',
				'av1', 'av01.0.05M.08', 'av01.0.04M.08',
				'mpeg4', 'mp4v.20.8',
				'mpeg2', 'mp2v',
				'vc1', 'vc-1', 'wvc1'
			];

			// Test using canPlayType
			for (const codec of testCodecs) {
				try {
					const canPlay = document.createElement('video').canPlayType(
						`video/mp4; codecs="${codec}"`
					);
					if (canPlay === 'probably' || canPlay === 'maybe') {
						codecs.add(codec.toLowerCase().split('.')[0]);
						this._log('debug', `Video codec supported: ${codec}`);
					}
				} catch (e) {
					// Ignore errors
				}
			}

			// Test using mediaCapabilities if available
			if (navigator.mediaCapabilities && navigator.mediaCapabilities.decodingInfo) {
				for (const codec of testCodecs) {
					try {
						const result = await navigator.mediaCapabilities.decodingInfo({
							type: 'file',
							video: {
								contentType: `video/${codec}`,
								width: 1920,
								height: 1080,
								bitrate: 5000000,
								framerate: 30
							}
						});

						if (result.supported) {
							codecs.add(codec.toLowerCase().split('.')[0]);
							this._log('debug', `Video codec supported via mediaCapabilities: ${codec}`);
						}
					} catch (e) {
						// Ignore errors
					}
				}
			}

			// Normalize codec names
			const normalizedCodecs = Array.from(codecs).map(c => {
				if (c.startsWith('avc')) return 'h264';
				if (c.startsWith('hev') || c.startsWith('hvc')) return 'hevc';
				if (c.startsWith('vp09')) return 'vp9';
				if (c.startsWith('av01')) return 'av1';
				if (c.startsWith('mp2v')) return 'mpeg2';
				if (c.startsWith('mp4v')) return 'mpeg4';
				return c;
			});

			// Remove duplicates
			return [...new Set(normalizedCodecs)];
		}

		/**
		 * Detects audio codec support using canPlayType and mediaCapabilities
		 * @private
		 * @returns {Promise<Array>} List of supported audio codecs
		 */
		async _detectAudioCodecs() {
			const codecs = new Set();
			const testCodecs = [
				'aac', 'mp4a.40.2', 'mp4a.40.5',
				'mp3', 'audio/mpeg',
				'flac', 'audio/flac', 'audio/x-flac',
				'opus', 'audio/opus', 'opus',
				'ac3', 'audio/ac3', 'ac-3',
				'eac3', 'audio/eac3', 'ec-3',
				'dts', 'audio/vnd.dts',
				'truehd', 'audio/true-hd',
				'alac', 'audio/alac', 'alac',
				'vorbis', 'audio/vorbis', 'audio/ogg; codecs="vorbis"'
			];

			// Test using canPlayType
			for (const codec of testCodecs) {
				try {
					const canPlay = document.createElement('audio').canPlayType(codec);
					if (canPlay === 'probably' || canPlay === 'maybe') {
						const normalized = codec.toLowerCase().split('.')[0].split('/')[0];
						codecs.add(normalized);
						this._log('debug', `Audio codec supported: ${codec}`);
					}
				} catch (e) {
					// Ignore errors
				}
			}

			// Test using mediaCapabilities if available
			if (navigator.mediaCapabilities && navigator.mediaCapabilities.decodingInfo) {
				const audioTestCodecs = [
					'aac', 'mp3', 'opus', 'flac', 'ac3', 'eac3', 'vorbis'
				];

				for (const codec of audioTestCodecs) {
					try {
						const result = await navigator.mediaCapabilities.decodingInfo({
							type: 'file',
							audio: {
								contentType: `audio/${codec}`,
								channels: 2,
								bitrate: 128000,
								samplerate: 48000
							}
						});

						if (result.supported) {
							codecs.add(codec);
							this._log('debug', `Audio codec supported via mediaCapabilities: ${codec}`);
						}
					} catch (e) {
						// Ignore errors
					}
				}
			}

			return Array.from(codecs);
		}

		/**
		 * Detects container format support
		 * @private
		 * @returns {Promise<Array>} List of supported container formats
		 */
		async _detectContainers() {
			const containers = new Set();
			const testContainers = [
				{ mime: 'video/mp4', formats: ['mp4', 'm4v', 'm4a'] },
				{ mime: 'video/webm', formats: ['webm'] },
				{ mime: 'video/ogg', formats: ['ogv', 'ogg'] },
				{ mime: 'video/quicktime', formats: ['mov'] },
				{ mime: 'video/x-msvideo', formats: ['avi'] },
				{ mime: 'video/x-matroska', formats: ['mkv'] }
			];

			for (const { mime, formats } of testContainers) {
				try {
					const canPlay = document.createElement('video').canPlayType(mime);
					if (canPlay === 'probably' || canPlay === 'maybe') {
						formats.forEach(f => containers.add(f));
						this._log('debug', `Container supported: ${mime}`);
					}
				} catch (e) {
					// Ignore errors
				}
			}

			return Array.from(containers);
		}

		/**
		 * Detects maximum video resolution support
		 * @private
		 * @returns {Promise<Object>} Maximum width and height
		 */
		async _detectMaxResolution() {
			const maxRes = { width: 0, height: 0 };

			// Check screen resolution as baseline
			const screenRes = {
				width: window.screen.width * window.devicePixelRatio,
				height: window.screen.height * window.devicePixelRatio
			};

			// Test mediaCapabilities for resolution support
			if (navigator.mediaCapabilities && navigator.mediaCapabilities.decodingInfo) {
				const resolutions = [
					{ w: 3840, h: 2160 }, // 4K
					{ w: 2560, h: 1440 }, // 1440p
					{ w: 1920, h: 1080 }, // 1080p
					{ w: 1280, h: 720 },  // 720p
					{ w: 854, h: 480 },   // 480p
					{ w: 640, h: 360 }    // 360p
				];

				for (const res of resolutions) {
					try {
						const result = await navigator.mediaCapabilities.decodingInfo({
							type: 'file',
							video: {
								contentType: 'video/mp4; codecs="avc1.640028"',
								width: res.w,
								height: res.h,
								bitrate: 8000000,
								framerate: 30
							}
						});

						if (result.supported && result.powerEfficient) {
							maxRes.width = res.w;
							maxRes.height = res.h;
							this._log('debug', `Resolution supported: ${res.w}x${res.h}`);
						}
					} catch (e) {
						// Continue testing
					}
				}
			}

			// If no resolution detected via mediaCapabilities, use screen resolution
			if (maxRes.width === 0) {
				maxRes.width = screenRes.width;
				maxRes.height = screenRes.height;
			}

			return maxRes;
		}

		/**
		 * Detects HDR support
		 * @private
		 * @returns {Promise<boolean>} Whether HDR is supported
		 */
		async _detectHDRSupport() {
			if (navigator.mediaCapabilities && navigator.mediaCapabilities.decodingInfo) {
				try {
					const result = await navigator.mediaCapabilities.decodingInfo({
						type: 'file',
						video: {
							contentType: 'video/mp4; codecs="hev1.1.6.L153.B0"',
							width: 3840,
							height: 2160,
							bitrate: 25000000,
							framerate: 30,
							hdrMetadataType: 'smpte2086'
						}
					});

					return result.supported;
				} catch (e) {
					// HDR not supported
				}
			}

			// Check for HDR media type support
			try {
				const canPlayHDR = document.createElement('video').canPlayType(
					'video/mp4; codecs="hev1.1.6.L153.B0"; profiles="main10"'
				);
				return canPlayHDR === 'probably';
			} catch (e) {
				return false;
			}
		}

		/**
		 * Detects 10-bit color depth support
		 * @private
		 * @returns {Promise<boolean>} Whether 10-bit is supported
		 */
		async _detect10BitSupport() {
			if (navigator.mediaCapabilities && navigator.mediaCapabilities.decodingInfo) {
				try {
					const result = await navigator.mediaCapabilities.decodingInfo({
						type: 'file',
						video: {
							contentType: 'video/mp4; codecs="hev1.1.6.L153.B0"',
							width: 1920,
							height: 1080,
							bitrate: 5000000,
							framerate: 30,
							colorDepth: 10,
						}
					});

					return result.supported;
				} catch (e) {
					return false;
				}
			}

			return false;
		}

		/**
		 * Detects DRM support
		 * @private
		 * @returns {Promise<Object>} DRM capabilities
		 */
		async _detectDRMSupport() {
			const drmSupport = {
				fairplay: false,
				widevine: false,
				playready: false,
				clearkey: false
			};

			// Check for Encrypted Media Extensions support
			if (!window.MediaKeys) {
				return drmSupport;
			}

			const drmSystems = [
				{ name: 'fairplay', key: 'com.apple.fps' },
				{ name: 'widevine', key: 'org.w3.clearkey' },
				{ name: 'playready', key: 'com.microsoft.playready' },
				{ name: 'clearkey', key: 'org.w3.clearkey' }
			];

			for (const { name, key } of drmSystems) {
				try {
					const isSupported = await window.MediaKeys.isSupported(key);
					if (isSupported) {
						drmSupport[name] = true;
						this._log('debug', `DRM supported: ${name}`);
					}
				} catch (e) {
					// DRM not supported
				}
			}

			return drmSupport;
		}

		/**
		 * Detects maximum bitrate support
		 * @private
		 * @returns {Promise<number>} Maximum bitrate in bits per second
		 */
		async _detectMaxBitrate() {
			// Default estimates based on platform
			const platform = this._detectPlatform();
			const baseBitrates = {
				'ios': 50000000,
				'tvos': 100000000,
				'android': 50000000,
				'chromecast': 25000000,
				'roku': 30000000,
				'firetv': 40000000,
				'xbox': 100000000,
				'playstation': 80000000,
				'apple_tv': 100000000,
				'windows': 100000000,
				'macos': 100000000,
				'linux': 100000000,
				'web': 50000000,
				'chromeos': 50000000
			};

			// Test actual network speed if possible
			if (navigator.connection) {
				const downlink = navigator.connection.downlink || 50; // Mbps
				const estimated = downlink * 1000000 * 0.8; // 80% of connection speed

				return Math.min(estimated, baseBitrates[platform] || 50000000);
			}

			return baseBitrates[platform] || 50000000;
		}

		/**
		 * Detects Dolby Vision support
		 * @private
		 * @returns {Promise<boolean>} Whether Dolby Vision is supported
		 */
		async _detectDolbyVisionSupport() {
			if (navigator.mediaCapabilities && navigator.mediaCapabilities.decodingInfo) {
				try {
					const result = await navigator.mediaCapabilities.decodingInfo({
						type: 'file',
						video: {
							contentType: 'video/mp4; codecs="dvh1.05.06"',
							width: 3840,
							height: 2160,
							bitrate: 25000000,
							framerate: 30
						}
					});

					return result.supported;
				} catch (e) {
					return false;
				}
			}

			return false;
		}

		/**
		 * Detects Dolby Atmos support
		 * @private
		 * @returns {Promise<boolean>} Whether Dolby Atmos is supported
		 */
		async _detectDolbyAtmosSupport() {
			// Check for AC3/EAC3 support as base requirement
			const audioCodecs = await this._detectAudioCodecs();
			const hasAC3 = audioCodecs.includes('ac3') || audioCodecs.includes('eac3');

			if (!hasAC3) {
				return false;
			}

			// Check mediaCapabilities for Atmos
			if (navigator.mediaCapabilities && navigator.mediaCapabilities.decodingInfo) {
				try {
					const result = await navigator.mediaCapabilities.decodingInfo({
						type: 'file',
						audio: {
							contentType: 'audio/eac3',
							channels: 8,
							bitrate: 768000,
							samplerate: 48000
						}
					});

					return result.supported;
				} catch (e) {
					return false;
				}
			}

			return false;
		}

		/**
		 * Detects DTS support
		 * @private
		 * @returns {Promise<boolean>} Whether DTS is supported
		 */
		async _detectDTSSupport() {
			const audioCodecs = await this._detectAudioCodecs();
			return audioCodecs.includes('dts');
		}

		/**
		 * Collects all device capabilities
		 * @private
		 * @returns {Promise<Object>} Complete device capabilities
		 */
		async _collectCapabilities() {
			this._probeStartTime = Date.now();
			this._isProbing = true;

			try {
				this._reportProgress(5, 'detecting_platform');

				const platform = this._detectPlatform();
				const browser = this._detectBrowser();

				this._reportProgress(15, 'detecting_codecs');
				const videoCodecs = await this._detectVideoCodecs();
				const audioCodecs = await this._detectAudioCodecs();

				this._reportProgress(30, 'detecting_containers');
				const containerFormats = await this._detectContainers();

				this._reportProgress(40, 'detecting_resolution');
				const maxResolution = await this._detectMaxResolution();

				this._reportProgress(50, 'detecting_hdr');
				const supportsHDR = await this._detectHDRSupport();

				this._reportProgress(60, 'detecting_10bit');
				const supports10Bit = await this._detect10BitSupport();

				this._reportProgress(70, 'detecting_drm');
				const drmSupport = await this._detectDRMSupport();

				this._reportProgress(80, 'detecting_bitrate');
				const maxBitrate = await this._detectMaxBitrate();

				this._reportProgress(85, 'detecting_dolby_vision');
				const supportsDolbyVision = await this._detectDolbyVisionSupport();

				this._reportProgress(90, 'detecting_dolby_atmos');
				const supportsDolbyAtmos = await this._detectDolbyAtmosSupport();

				this._reportProgress(95, 'detecting_dts');
				const supportsDTS = await this._detectDTSSupport();

				const capabilities = {
					deviceId: this.deviceId,
					platform: platform,
					browser: browser.name,
					browserVersion: browser.version,
					userAgent: navigator.userAgent,
					videoCodecs: videoCodecs,
					audioCodecs: audioCodecs,
					containerFormats: containerFormats,
					maxWidth: maxResolution.width,
					maxHeight: maxResolution.height,
					maxBitrate: maxBitrate,
					supportsHDR: supportsHDR,
					supports10Bit: supports10Bit,
					supportsDolbyVision: supportsDolbyVision,
					supportsDolbyAtmos: supportsDolbyAtmos,
					supportsDTS: supportsDTS,
					drmSupport: drmSupport,
					screenWidth: window.screen.width,
					screenHeight: window.screen.height,
					devicePixelRatio: window.devicePixelRatio || 1,
					online: navigator.onLine,
					connectionType: navigator.connection ? navigator.connection.effectiveType : null,
					timestamp: new Date().toISOString()
				};

				this._detectedCapabilities = capabilities;
				this._reportProgress(100, 'complete', capabilities);

				return capabilities;
			} catch (error) {
				this._log('error', `Capability detection failed: ${error.message}`);
				throw error;
			} finally {
				this._isProbing = false;
			}
		}

		/**
		 * Sends probe report to the server
		 * @private
		 * @param {Object} capabilities - Device capabilities
		 * @returns {Promise<Object>} Server response
		 */
		async _sendReport(capabilities) {
			const url = `${this.serverUrl}/api/v1/probe/report`;

			const payload = {
				capabilities: capabilities,
				source: 'client_probe',
				version: '1.0.0'
			};

			let lastError = null;

			for (let attempt = 0; attempt <= this.maxRetries; attempt++) {
				try {
					this._log('debug', `Sending report (attempt ${attempt + 1}/${this.maxRetries + 1})`);

					const response = await fetch(url, {
						method: 'POST',
						headers: {
							'Content-Type': 'application/json',
							'Accept': 'application/json'
						},
						body: JSON.stringify(payload),
						signal: AbortSignal.timeout(this.timeout)
					});

					if (!response.ok) {
						const errorText = await response.text().catch(() => '');
						throw new Error(`HTTP ${response.status}: ${errorText}`);
					}

					const data = await response.json().catch(() => ({}));
					this._log('debug', 'Report sent successfully', data);

					return {
						success: true,
						data: data,
						attempts: attempt + 1
					};
				} catch (error) {
					lastError = error;
					this._log('error', `Report send failed: ${error.message}`);

					if (attempt < this.maxRetries) {
						// Exponential backoff
						const delay = Math.min(1000 * Math.pow(2, attempt), 10000);
						await new Promise(resolve => setTimeout(resolve, delay));
					}
				}
			}

			throw new Error(`Failed to send report after ${this.maxRetries + 1} attempts: ${lastError.message}`);
		}

		/**
		 * Runs the full probe sequence
		 * @public
		 * @returns {Promise<Object>} Probe result
		 */
		async probe() {
			if (this._isProbing) {
				throw new Error('Probe already in progress');
			}

			try {
				this._log('info', 'Starting device probe');
				this._reportProgress(0, 'starting');

				// Collect capabilities
				const capabilities = await this._collectCapabilities();

				// Send report
				const result = await this._sendReport(capabilities);

				// Call success callback if provided
				if (this.reportSuccess && typeof this.reportSuccess === 'function') {
					this.reportSuccess(capabilities, result.data);
				}

				// Call complete callback
				if (this.onComplete && typeof this.onComplete === 'function') {
					this.onComplete({
						success: true,
						capabilities: capabilities,
						result: result,
						duration: Date.now() - this._probeStartTime
					});
				}

				this._log('info', 'Probe completed successfully');

				return {
					success: true,
					capabilities: capabilities,
					result: result,
					duration: Date.now() - this._probeStartTime
				};
			} catch (error) {
				this._log('error', `Probe failed: ${error.message}`);

				// Call error callback if provided
				if (this.onError && typeof this.onError === 'function') {
					this.onError(error);
				}

				// Call failure callback if provided
				if (this.reportFailure && typeof this.reportFailure === 'function') {
					this.reportFailure(error);
				}

				if (this.onComplete && typeof this.onComplete === 'function') {
					this.onComplete({
						success: false,
						error: error.message,
						duration: Date.now() - this._probeStartTime
					});
				}

				return {
					success: false,
					error: error.message,
					duration: Date.now() - this._probeStartTime
				};
			}
		}

		/**
		 * Reports playback success
		 * @public
		 * @param {Object} playbackInfo - Playback information
		 */
		reportPlaybackSuccess(playbackInfo) {
			if (this.reportSuccess && typeof this.reportSuccess === 'function') {
				this.reportSuccess({ type: 'playback_success', ...playbackInfo });
			}

			// Send to server
			this._sendPlaybackReport('success', playbackInfo).catch(err => {
				this._log('error', `Playback success report failed: ${err.message}`);
			});
		}

		/**
		 * Reports playback failure
		 * @public
		 * @param {Object} failureInfo - Failure information
		 */
		reportPlaybackFailure(failureInfo) {
			if (this.reportFailure && typeof this.reportFailure === 'function') {
				this.reportFailure({ type: 'playback_failure', ...failureInfo });
			}

			// Send to server
			this._sendPlaybackReport('failure', failureInfo).catch(err => {
				this._log('error', `Playback failure report failed: ${err.message}`);
			});
		}

		/**
		 * Sends playback report to server
		 * @private
		 * @param {string} type - Report type ('success' or 'failure')
		 * @param {Object} info - Playback information
		 */
		async _sendPlaybackReport(type, info) {
			const url = `${this.serverUrl}/api/v1/probe/playback/${type}`;

			const payload = {
				deviceId: this.deviceId,
				capabilities: this._detectedCapabilities,
				playbackInfo: info,
				timestamp: new Date().toISOString()
			};

			try {
				await fetch(url, {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						'Accept': 'application/json'
					},
					body: JSON.stringify(payload),
					signal: AbortSignal.timeout(this.timeout)
				});
			} catch (error) {
				this._log('error', `Playback report failed: ${error.message}`);
			}
		}

		/**
		 * Gets the last detected capabilities
		 * @public
		 * @returns {Object|null} Detected capabilities or null
		 */
		getCapabilities() {
			return this._detectedCapabilities;
		}

		/**
		 * Sets a custom device ID
		 * @public
		 * @param {string} deviceId - Custom device ID
		 */
		setDeviceId(deviceId) {
			this.deviceId = deviceId;
		}

		/**
		 * Checks if a specific codec is supported
		 * @public
		 * @param {string} codec - Codec to check
		 * @param {string} [type='video'] - Media type ('video' or 'audio')
		 * @returns {Promise<boolean>} Whether codec is supported
		 */
		async isCodecSupported(codec, type = 'video') {
			if (type === 'video') {
				const codecs = await this._detectVideoCodecs();
				return codecs.some(c => c.toLowerCase().includes(codec.toLowerCase()));
			} else {
				const codecs = await this._detectAudioCodecs();
				return codecs.some(c => c.toLowerCase().includes(codec.toLowerCase()));
			}
		}

		/**
		 * Checks if HDR is supported
		 * @public
		 * @returns {Promise<boolean>} Whether HDR is supported
		 */
		async isHDRSupported() {
			return this._detectHDRSupport();
		}

		/**
		 * Checks if 4K resolution is supported
		 * @public
		 * @returns {Promise<boolean>} Whether 4K is supported
		 */
		async is4KSupported() {
			const res = await this._detectMaxResolution();
			return res.width >= 3840 && res.height >= 2160;
		}

		/**
		 * Gets a quick capability summary
		 * @public
		 * @returns {Promise<Object>} Capability summary
		 */
		async getSummary() {
			const capabilities = await this._collectCapabilities();

			return {
				platform: capabilities.platform,
				videoCodecs: capabilities.videoCodecs.length,
				audioCodecs: capabilities.audioCodecs.length,
				maxResolution: `${capabilities.maxWidth}x${capabilities.maxHeight}`,
				supportsHDR: capabilities.supportsHDR,
				supports4K: capabilities.maxWidth >= 3840,
				drmSupport: Object.keys(capabilities.drmSupport).filter(drm => capabilities.drmSupport[drm])
			};
		}
	}

	/**
	 * Creates a new TenkileProbe instance
	 * @param {Object} options - Configuration options
	 * @returns {TenkileProbe} Probe instance
	 */
	function createProbe(options = {}) {
		return new TenkileProbe(options);
	}

	/**
	 * Quick probe helper - runs probe with default options
	 * @param {Object} options - Configuration options
	 * @returns {Promise<Object>} Probe result
	 */
	async function quickProbe(options = {}) {
		const probe = createProbe(options);
		return await probe.probe();
	}

	// Export for different module systems
	if (typeof module !== 'undefined' && module.exports) {
		// CommonJS
		module.exports = {
			TenkileProbe,
			createProbe,
			quickProbe
		};
	} else if (typeof define === 'function' && define.amd) {
		// AMD
		define([], function () {
			return {
				TenkileProbe: TenkileProbe,
				createProbe: createProbe,
				quickProbe: quickProbe
			};
		});
	} else {
		// Global
		global.TenkileProbe = TenkileProbe;
		global.createProbe = createProbe;
		global.quickProbe = quickProbe;
	}

})(typeof window !== 'undefined' ? window : this);

/**
 * @typedef {Object} CapabilityReport
 * @property {string} deviceId - Unique device identifier
 * @property {string} platform - Platform identifier (ios, android, windows, etc.)
 * @property {string} browser - Browser name
 * @property {string} browserVersion - Browser version
 * @property {string} userAgent - Full user agent string
 * @property {string[]} videoCodecs - List of supported video codecs
 * @property {string[]} audioCodecs - List of supported audio codecs
 * @property {string[]} containerFormats - List of supported container formats
 * @property {number} maxWidth - Maximum supported video width
 * @property {number} maxHeight - Maximum supported video height
 * @property {number} maxBitrate - Maximum supported bitrate in bps
 * @property {boolean} supportsHDR - Whether HDR is supported
 * @property {boolean} supports10Bit - Whether 10-bit color is supported
 * @property {boolean} supportsDolbyVision - Whether Dolby Vision is supported
 * @property {boolean} supportsDolbyAtmos - Whether Dolby Atmos is supported
 * @property {boolean} supportsDTS - Whether DTS audio is supported
 * @property {Object} drmSupport - DRM system support
 * @property {boolean} drmSupport.fairplay - FairPlay support
 * @property {boolean} drmSupport.widevine - Widevine support
 * @property {boolean} drmSupport.playready - PlayReady support
 * @property {boolean} drmSupport.clearkey - ClearKey support
 * @property {number} screenWidth - Physical screen width
 * @property {number} screenHeight - Physical screen height
 * @property {number} devicePixelRatio - Device pixel ratio
 * @property {boolean} online - Whether device is online
 * @property {string|null} connectionType - Network connection type
 * @property {string} timestamp - Detection timestamp (ISO 8601)
 */

/**
 * @typedef {Object} ProgressCallback
 * @property {number} progress - Progress percentage (0-100)
 * @property {string} stage - Current probe stage
 * @property {Object} details - Additional stage details
 * @property {number} elapsedMs - Elapsed time in milliseconds
 */

/**
 * @typedef {Object} ProbeResult
 * @property {boolean} success - Whether probe succeeded
 * @property {CapabilityReport} [capabilities] - Detected capabilities (if successful)
 * @property {string} [error] - Error message (if failed)
 * @property {number} duration - Probe duration in milliseconds
 * @property {Object} [result] - Server response (if sent)
 */

/**
 * PlaybackTracker - Tracks HTML5 video playback events and reports outcomes
 * @class
 */
class PlaybackTracker {
	/**
	 * Creates a new PlaybackTracker instance
	 * @param {Object} options - Configuration options
	 * @param {string} options.serverUrl - Base URL for the Tenkile server
	 * @param {string} options.deviceId - Device ID for reporting
	 * @param {Object} options.capabilities - Device capabilities from TenkileProbe
	 * @param {number} [options.maxRetries=3] - Maximum retry attempts
	 * @param {number} [options.timeout=5000] - Request timeout in milliseconds
	 * @param {number} [options.bufferThreshold=0.5] - Buffer health threshold (0-1)
	 * @param {number} [options.stallThreshold=3000] - Stall detection threshold in ms
	 */
	constructor(options = {}) {
		this.serverUrl = options.serverUrl || '';
		this.deviceId = options.deviceId || '';
		this.capabilities = options.capabilities || {};
		this.maxRetries = options.maxRetries || 3;
		this.timeout = options.timeout || 5000;
		this.bufferThreshold = options.bufferThreshold || 0.5;
		this.stallThreshold = options.stallThreshold || 3000;

		// State
		this._currentVideo = null;
		this._mediaInfo = null;
		// Use circular buffer for event history to avoid memory leak
		this._eventHistory = {
			buffer: new Array(100),
			index: 0,
			count: 0,
			push(event) {
				this.buffer[this.index] = event;
				this.index = (this.index + 1) % 100;
				if (this.count < 100) this.count++;
			},
			getAll() {
				const result = [];
				const start = this.count < 100 ? 0 : this.index;
				for (let i = 0; i < this.count; i++) {
					result.push(this.buffer[(start + i) % 100]);
				}
				return result;
			}
		};
		this._bufferingEvents = [];
		this._startTime = null;
		this._lastProgressTime = null;
		this._stallTimer = null;
		this._retryCount = 0;
		this._maxRetries = 3;
		this._retryDelay = 1000;
		this._isTracking = false;

		// Statistics
		this._stats = {
			totalPlaybacks: 0,
			successfulPlaybacks: 0,
			failedPlaybacks: 0,
			totalBufferingMs: 0,
			bufferingEvents: 0
		};

		// Bound event handlers
		this._boundHandlers = {};
	}

	/**
	 * Starts tracking a video element
	 * @param {HTMLVideoElement} video - Video element to track
	 * @param {Object} mediaInfo - Media information (codec, container, etc.)
	 */
	startTracking(video, mediaInfo = {}) {
		if (this._isTracking) {
			this.stopTracking();
		}

		this._currentVideo = video;
		this._mediaInfo = {
			mediaId: mediaInfo.mediaId || '',
			videoCodec: mediaInfo.videoCodec || this._extractCodec(mediaInfo),
			audioCodec: mediaInfo.audioCodec || '',
			container: mediaInfo.container || this._extractContainer(video.src),
			resolution: mediaInfo.resolution || `${video.videoWidth || 0}x${video.videoHeight || 0}`,
			bitrate: mediaInfo.bitrate || 0,
			duration: video.duration || 0
		};
		this._startTime = Date.now();
		this._lastProgressTime = Date.now();
		this._isTracking = true;
		this._stats.totalPlaybacks++;

		// Bind event handlers
		this._bindHandlers();

		console.log('[PlaybackTracker] Started tracking', this._mediaInfo);
	}

	/**
	 * Stops tracking the current video
	 */
	stopTracking() {
		if (!this._isTracking) return;

		// Unbind event handlers
		this._unbindHandlers();

		// Clear stall timer
		if (this._stallTimer) {
			clearTimeout(this._stallTimer);
			this._stallTimer = null;
		}

		this._currentVideo = null;
		this._mediaInfo = null;
		this._isTracking = false;

		console.log('[PlaybackTracker] Stopped tracking');
	}

	/**
	 * Reports a successful playback
	 * @param {Object} additionalInfo - Additional information to include
	 */
	reportSuccess(additionalInfo = {}) {
		const duration = Date.now() - (this._startTime || Date.now());
		const bufferingDuration = this._bufferingEvents.reduce((sum, e) => sum + e.duration, 0);

		const feedback = {
			mediaId: this._mediaInfo?.mediaId || '',
			outcome: 'success',
			durationSeconds: duration / 1000,
			bufferDurationSeconds: bufferingDuration / 1000,
			networkQuality: this._getNetworkQuality(),
			...this._getMediaDetails(),
			...additionalInfo
		};

		this._sendFeedback(feedback);
		this._stats.successfulPlaybacks++;

		console.log('[PlaybackTracker] Reported success', feedback);
	}

	/**
	 * Reports a failed playback
	 * @param {string} outcome - Failure outcome type
	 * @param {string} errorCode - Error code
	 * @param {string} errorMessage - Error message
	 * @param {Object} additionalInfo - Additional information
	 */
	reportFailure(outcome, errorCode, errorMessage, additionalInfo = {}) {
		const duration = Date.now() - (this._startTime || Date.now());
		const bufferingDuration = this._bufferingEvents.reduce((sum, e) => sum + e.duration, 0);

		const feedback = {
			mediaId: this._mediaInfo?.mediaId || '',
			outcome: outcome,
			errorCode: errorCode,
			errorMessage: errorMessage,
			durationSeconds: duration / 1000,
			bufferDurationSeconds: bufferingDuration / 1000,
			networkQuality: this._getNetworkQuality(),
			...this._getMediaDetails(),
			...additionalInfo
		};

		this._sendFeedback(feedback);
		this._stats.failedPlaybacks++;

		console.log('[PlaybackTracker] Reported failure', feedback);
	}

	/**
	 * Gets current tracking statistics
	 * @returns {Object} Statistics
	 */
	getStats() {
		return {
			...this._stats,
			successRate: this._stats.totalPlaybacks > 0 
				? this._stats.successfulPlaybacks / this._stats.totalPlaybacks 
				: 0,
			averageBufferingMs: this._stats.bufferingEvents > 0 
				? this._stats.totalBufferingMs / this._stats.bufferingEvents 
				: 0
		};
	}

	/**
	 * Resets tracking statistics
	 */
	resetStats() {
		this._stats = {
			totalPlaybacks: 0,
			successfulPlaybacks: 0,
			failedPlaybacks: 0,
			totalBufferingMs: 0,
			bufferingEvents: 0
		};
	}

	// --- Private Methods ---

	_bindHandlers() {
		if (!this._currentVideo) return;

		const video = this._currentVideo;

		// Playback events
		this._boundHandlers.playing = () => this._onPlaying();
		this._boundHandlers.pause = () => this._onPause();
		this._boundHandlers.ended = () => this._onEnded();
		this._boundHandlers.waiting = () => this._onWaiting();
		this._boundHandlers.canplay = () => this._onCanPlay();
		this._boundHandlers.canplaythrough = () => this._onCanPlayThrough();

		// Error events
		this._boundHandlers.error = () => this._onError();
		this._boundHandlers.stalled = () => this._onStalled();

		// Progress events
		this._boundHandlers.progress = () => this._onProgress();
		this._boundHandlers.timeupdate = () => this._onTimeUpdate();
		this._boundHandlers.loadedmetadata = () => this._onLoadedMetadata();

		// Network events
		this._boundHandlers.suspend = () => this._onSuspend();
		this._boundHandlers.abort = () => this._onAbort();
		this._boundHandlers.emptied = () => this._onEmptied();

		// Rate change
		this._boundHandlers.ratechange = () => this._onRateChange();

		// Add listeners
		for (const [event, handler] of Object.entries(this._boundHandlers)) {
			video.addEventListener(event, handler);
		}
	}

	_unbindHandlers() {
		if (!this._currentVideo) return;

		const video = this._currentVideo;
		for (const [event, handler] of Object.entries(this._boundHandlers)) {
			video.removeEventListener(event, handler);
		}
		this._boundHandlers = {};
	}

	_onPlaying() {
		// Clear stall timer
		if (this._stallTimer) {
			clearTimeout(this._stallTimer);
			this._stallTimer = null;
		}

		// Record event
		this._recordEvent('playing');

		// Clear any previous buffering event
		const currentBuffering = this._bufferingEvents[this._bufferingEvents.length - 1];
		if (currentBuffering && !currentBuffering.endTime) {
			currentBuffering.endTime = Date.now();
			currentBuffering.duration = currentBuffering.endTime - currentBuffering.startTime;
			this._stats.totalBufferingMs += currentBuffering.duration;
		}
	}

	_onPause() {
		this._recordEvent('pause');
	}

	_onEnded() {
		this._recordEvent('ended');
		this.reportSuccess({ completed: true });
	}

	_onWaiting() {
		// Start buffering tracking
		const bufferingEvent = {
			type: 'buffering',
			startTime: Date.now(),
			endTime: null,
			duration: 0
		};
		this._bufferingEvents.push(bufferingEvent);
		this._stats.bufferingEvents++;

		// Start stall timer
		this._stallTimer = setTimeout(() => {
			this._onStallTimeout();
		}, this.stallThreshold);
	}

	_onCanPlay() {
		this._recordEvent('canplay');
	}

	_onCanPlayThrough() {
		this._recordEvent('canplaythrough');
	}

	_onError() {
		const error = this._currentVideo?.error;
		if (!error) return;

		let outcome = 'unknown';
		let errorCode = 'UNKNOWN';
		let errorMessage = 'Unknown error';

		switch (error.code) {
			case MediaError.MEDIA_ERR_ABORTED:
				outcome = 'network_error';
				errorCode = 'MEDIA_ERR_ABORTED';
				errorMessage = 'Playback aborted by user';
				break;
			case MediaError.MEDIA_ERR_NETWORK:
				outcome = 'network_error';
				errorCode = 'MEDIA_ERR_NETWORK';
				errorMessage = 'Network error occurred';
				break;
			case MediaError.MEDIA_ERR_DECODE:
				outcome = 'decoding_failed';
				errorCode = 'MEDIA_ERR_DECODE';
				errorMessage = 'Media decoding error';
				break;
			case MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED:
				outcome = 'unsupported_format';
				errorCode = 'MEDIA_ERR_SRC_NOT_SUPPORTED';
				errorMessage = 'Media format not supported';
				break;
		}

		this.reportFailure(outcome, errorCode, errorMessage);
	}

	_onStalled() {
		this._recordEvent('stalled');
		
		// Treat as temporary stall - don't report immediately
		// The stall timer will handle reporting if it persists
	}

	_onStallTimeout() {
		// Stall persisted - report as buffering issue
		this.reportFailure('buffering', 'STALL_TIMEOUT', 'Playback stalled for too long');
	}

	_onProgress() {
		// Check buffer health
		if (this._currentVideo && this._currentVideo.buffered.length > 0) {
			const bufferedEnd = this._currentVideo.buffered.end(this._currentVideo.buffered.length - 1);
			const currentTime = this._currentVideo.currentTime;
			const duration = this._currentVideo.duration;
			
			const bufferAhead = bufferedEnd - currentTime;
			const bufferHealth = duration > 0 ? bufferAhead / duration : 0;

			if (bufferHealth < this.bufferThreshold) {
				// Low buffer health - might indicate issues
				console.log('[PlaybackTracker] Low buffer health:', bufferHealth);
			}
		}
	}

	_onTimeUpdate() {
		this._lastProgressTime = Date.now();

		// Clear stall timer if we're making progress
		if (this._stallTimer) {
			clearTimeout(this._stallTimer);
			this._stallTimer = null;
		}
	}

	_onLoadedMetadata() {
		this._recordEvent('loadedmetadata');

		// Update media info with actual dimensions
		if (this._currentVideo) {
			this._mediaInfo = {
				...this._mediaInfo,
				resolution: `${this._currentVideo.videoWidth}x${this._currentVideo.videoHeight}`
			};
		}
	}

	_onSuspend() {
		this._recordEvent('suspend');
	}

	_onAbort() {
		this.reportFailure('network_error', 'ABORT', 'Playback aborted');
	}

	_onEmptied() {
		this._recordEvent('emptied');
	}

	_onRateChange() {
		this._recordEvent('ratechange');
	}

	_recordEvent(type) {
		this._eventHistory.push({
			type,
			time: Date.now()
		});
	}

	_extractCodec(mediaInfo) {
		// Try to extract codec from capabilities
		if (this.capabilities?.videoCodecs?.length > 0) {
			return this.capabilities.videoCodecs[0];
		}
		return mediaInfo.codec || '';
	}

	_extractContainer(src) {
		if (!src) return '';
		const match = src.match(/\.([^.?#]+)(\?|$)/);
		return match ? match[1].toLowerCase() : '';
	}

	_getMediaDetails() {
		return {
			videoCodec: this._mediaInfo?.videoCodec || '',
			audioCodec: this._mediaInfo?.audioCodec || '',
			container: this._mediaInfo?.container || '',
			resolution: this._mediaInfo?.resolution || '',
			bitrate: this._mediaInfo?.bitrate || 0
		};
	}

	_getNetworkQuality() {
		if (!navigator.connection) return 'unknown';

		const connection = navigator.connection;
		const effectiveType = connection.effectiveType || 'unknown';

		// Map to qualitative levels
		const qualityMap = {
			'slow-2g': 'poor',
			'2g': 'poor',
			'3g': 'fair',
			'4g': 'good'
		};

		return qualityMap[effectiveType] || 'unknown';
	}

	async _sendFeedback(feedback) {
		const url = `${this.serverUrl}/api/v1/devices/${this.deviceId}/feedback`;

		const payload = {
			deviceId: this.deviceId,
			mediaId: feedback.mediaId,
			outcome: feedback.outcome,
			errorCode: feedback.errorCode || undefined,
			errorMessage: feedback.errorMessage || undefined,
			durationSeconds: feedback.durationSeconds,
			bufferDurationSeconds: feedback.bufferDurationSeconds,
			networkQuality: feedback.networkQuality,
			videoCodec: feedback.videoCodec,
			audioCodec: feedback.audioCodec,
			container: feedback.container,
			resolution: feedback.resolution,
			bitrate: feedback.bitrate
		};

		// Remove undefined values
		Object.keys(payload).forEach(key => {
			if (payload[key] === undefined) delete payload[key];
		});

		// Exponential backoff retry
		let lastError = null;
		for (let attempt = 0; attempt <= this.maxRetries; attempt++) {
			try {
				const response = await fetch(url, {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						'Accept': 'application/json'
					},
					body: JSON.stringify(payload),
					signal: AbortSignal.timeout(this.timeout)
				});

				if (!response.ok) {
					throw new Error(`HTTP ${response.status}`);
				}

				return await response.json();
			} catch (error) {
				lastError = error;
				console.log(`[PlaybackTracker] Feedback send failed (attempt ${attempt + 1}):`, error.message);

				if (attempt < this.maxRetries) {
					const delay = this._retryDelay * Math.pow(2, attempt);
					await new Promise(resolve => setTimeout(resolve, delay));
				}
			}
		}

		console.error('[PlaybackTracker] Failed to send feedback after retries:', lastError);
	}
}

/**
 * Creates a playback tracker for a video element
 * @param {HTMLVideoElement} video - Video element to track
 * @param {Object} options - Configuration options
 * @returns {PlaybackTracker} PlaybackTracker instance
 */
function createPlaybackTracker(video, options = {}) {
	return new PlaybackTracker(options);
}

// Export for different module systems
if (typeof module !== 'undefined' && module.exports) {
	module.exports = {
		TenkileProbe,
		createProbe,
		quickProbe,
		PlaybackTracker,
		createPlaybackTracker
	};
} else if (typeof define === 'function' && define.amd) {
	define([], function () {
		return {
			TenkileProbe: TenkileProbe,
			createProbe: createProbe,
			quickProbe: quickProbe,
			PlaybackTracker: PlaybackTracker,
			createPlaybackTracker: createPlaybackTracker
		};
	});
} else {
	// Global
	global.TenkileProbe = TenkileProbe;
	global.createProbe = createProbe;
	global.quickProbe = quickProbe;
	global.PlaybackTracker = PlaybackTracker;
	global.createPlaybackTracker = createPlaybackTracker;
}

// Polyfill for AbortSignal.timeout (older browsers)
if (!('timeout' in AbortSignal.prototype)) {
	AbortSignal.timeout = function(ms) {
		const controller = new AbortController();
		setTimeout(() => controller.abort(), ms);
		return controller.signal;
	};
}
