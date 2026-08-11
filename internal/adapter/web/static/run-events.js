// run-events.js (FR-R10): minimal browser-side consumer for the /ws
// endpoint's RunEvent push notifications (see internal/adapter/web/
// websocket_handler.go's RunEvent doc comment for the wire shape). Included
// on Job/Chore/History(run) detail pages so a Run's status/log, and the
// History notification badge, update live without a page reload.
//
// Deliberately dependency-free vanilla JS (no build step, no framework) —
// this is a minimal reference implementation, not a general real-time UI
// framework. Each detail page identifies what it cares about via data
// attributes on <body>:
//   - run_detail.html: data-run-id="<Run.ID>" — matches events by RunID.
//   - job_detail.html / chore_detail.html: data-source-type/data-source-id
//     — matches events by (SourceType, SourceID), since these pages don't
//     know their Job/Chore's current Run ID ahead of time.
// A page with none of these attributes (e.g. this script were ever included
// elsewhere) simply never matches a run_status/run_log event, but still
// updates the shared #notification-badge element for notification_badge
// events, which every page's nav.html renders.
(function () {
	"use strict";

	if (typeof window === "undefined" || !window.WebSocket) {
		return;
	}

	var body = document.body;
	var runId = body.getAttribute("data-run-id") || "";
	var sourceType = body.getAttribute("data-source-type") || "";
	var sourceId = body.getAttribute("data-source-id") || "";

	function matchesThisPage(event) {
		if (runId && event.runId === runId) {
			return true;
		}
		if (sourceType && sourceId && event.sourceType === sourceType && event.sourceId === sourceId) {
			return true;
		}
		return false;
	}

	function updateBadge(count) {
		var badge = document.getElementById("notification-badge");
		if (!badge) {
			return;
		}
		if (count > 0) {
			badge.textContent = String(count);
			badge.classList.remove("hidden");
		} else {
			badge.textContent = "0";
			badge.classList.add("hidden");
		}
	}

	function updateStatus(status) {
		var el = document.getElementById("run-status") || document.getElementById("job-status") || document.getElementById("chore-status");
		if (el) {
			el.textContent = status;
		}
	}

	function appendLog(chunk) {
		var el = document.getElementById("run-log");
		if (el && chunk) {
			el.textContent += chunk;
		}
	}

	function handleMessage(raw) {
		var event;
		try {
			event = JSON.parse(raw);
		} catch (e) {
			return;
		}
		if (!event || !event.type) {
			return;
		}

		if (event.type === "notification_badge") {
			updateBadge(event.unnotifiedCount || 0);
			return;
		}
		if (!matchesThisPage(event)) {
			return;
		}
		if (event.type === "run_status") {
			updateStatus(event.status);
		} else if (event.type === "run_log") {
			appendLog(event.logChunk);
		}
	}

	function connect() {
		var proto = window.location.protocol === "https:" ? "wss:" : "ws:";
		var ws = new WebSocket(proto + "//" + window.location.host + "/ws");
		ws.addEventListener("message", function (msg) {
			handleMessage(msg.data);
		});
		// No reconnect loop: this is a minimal reference implementation
		// (FR-R4's precedent — "at least one as a template the other three
		// can follow") — a dropped connection simply stops live updates
		// until the next page load, rather than adding backoff/retry
		// complexity out of scope for this pass.
	}

	connect();
})();
