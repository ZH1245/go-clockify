"use client";

import { useEffect } from "react";

import { api } from "@/lib/api";

// Just inside the server's two-minute timeout, even after hidden tabs are throttled
// to roughly one timer tick per minute.
const PING_INTERVAL_MS = 15_000;

// An open tab keeps its running timers alive by pinging; a tab that stops pinging lets
// the server time them out. That is why this pings on every page, whether or not a timer
// is running, and keeps pinging while the tab is hidden.
export function TimerHeartbeat() {
  useEffect(() => {
    // Failures are ignored: an unreachable or restarting API gets the next ping instead.
    const ping = () => {
      api.pingTimers().catch(() => {});
    };

    ping();
    const timer = setInterval(ping, PING_INTERVAL_MS);
    return () => clearInterval(timer);
  }, []);

  return null;
}
