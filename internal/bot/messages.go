package bot

// All texts the bot sends to users. %d and %q are filled in by fmt.Sprintf.
const (
	msgHelp = "Send a song name or a YouTube link. Tap a result to get the audio file."

	msgQueryTooShort  = "Please send at least %d characters."
	msgQueryTooLong   = "Please keep the search under %d characters."
	msgSearchCooldown = "Please wait a moment before searching again."
	msgSearching      = "Got it. Searching links..."
	msgSearchRetry    = "Search is slow. Retrying once..."
	msgSearchFailed   = "Could not search links right now. Please try again in a minute."
	msgNoResults      = "No links found for %q."
	msgResults        = "Found results for %q:"

	// Rich search results (rich.go).
	msgRichResultsTitle = "%s..."
	msgRichDownload     = "Download"
	msgPagePrev         = "‹ Back"
	msgPageNext         = "Next ›"
	msgPageExpired      = "These results expired. Search again."

	msgButtonEmpty   = "Nothing to do for this button."
	msgButtonExpired = "This download button expired. Search again."
	msgButtonStarted = "Starting download..."

	// /stats (stats.go), sent to admins only.
	msgStatsTitle          = "Stats"
	msgStatsToday          = "today"
	msgStatsWeek           = "7 days"
	msgStatsAll            = "all"
	msgStatsDownloads      = "Downloads"
	msgStatsFailedRow      = "failed"
	msgStatsUsers          = "Users"
	msgStatsNewUsers       = "new"
	msgStatsSearches       = "Searches"
	msgStatsDaily          = "Downloads, last %d days"
	msgStatsFailures       = "Failures:"
	msgStatsReasonSignIn   = "YouTube sign-in"
	msgStatsReasonTimeout  = "timeout"
	msgStatsReasonYouTube  = "YouTube"
	msgStatsReasonTelegram = "Telegram"
	msgStatsTime           = "Time:"
	msgStatsSearch         = "search"
	msgStatsDownload       = "download"
	msgStatsTimeNote       = "median / worst 5%"
	msgStatsFile           = "All %d events"
	msgStatsDisabled       = "Metrics are off: set BOT_EVENTS_FILE."
	msgStatsFailed         = "Could not build stats. See the bot log."
	msgAlertSignIn         = "YouTube asks to sign in: downloads are failing. The Cloudflare WARP exit IP is probably blocked."

	msgDownloadCooldown = "Please wait a moment before starting another conversion."
	msgDownloading      = "Got it. Downloading audio..."
	msgSendingAsFile    = "Telegram did not accept it as audio. Sending as file..."
	msgUploadFailed     = "Telegram upload failed. Please try again."
	msgDownloadTimeout  = "Download took too long and was stopped. Please try a shorter video."
	msgDownloadFailed   = "Could not download audio from YouTube. Please try again later."
	msgSentAudio        = "Done. Sent audio."
	msgSentFile         = "Done. Sent audio file."
)
