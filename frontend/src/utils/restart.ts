/**
 * Shared backend-restart health polling utility.
 *
 * Used by:
 * - Plugin management page (system/plugin/index.vue)
 * - Settings page (system/settings/index.vue)
 * - Version update dialog (VersionUpdateDialog.vue)
 *
 * All three used to have their own copy of the polling logic, and two of
 * them (settings, version dialog) had a classic race condition: they
 * declared "backend recovered" as soon as /health returned 200, even if
 * the old process was still alive (restart never happened). This shared
 * implementation requires the caller to observe a real outage before
 * accepting recovery, matching the plugin page's proven approach.
 */

interface PollHealthOptions {
  /** PID returned by the restart endpoint. If provided, the new process
   *  must have a different PID to be accepted as "recovered". */
  oldPid?: number
  /** True when the restart request itself failed (network error, 403, …).
   *  In that case we must see an outage before trusting any 200 response. */
  requestFailed?: boolean
  /** Max polling attempts (default 60). */
  maxRetries?: number
  /** Interval between polls in ms (default 1000). */
  interval?: number
  /** Called on each health probe (for progress UI). */
  onProbe?: (isDown: boolean, pid: number | undefined, attempt: number) => void
}

export interface PollHealthResult {
  recovered: boolean
  newPid?: number
  attempts: number
}

/**
 * Poll /api/v1/health until the backend comes back with a *new* process.
 *
 * Returns `{ recovered: true }` when two consecutive health checks from
 * the same new PID succeed, or `{ recovered: false }` after `maxRetries`.
 */
export async function pollBackendHealth(opts: PollHealthOptions = {}): Promise<PollHealthResult> {
  const {
    oldPid,
    requestFailed = false,
    maxRetries = 60,
    interval = 1000,
    onProbe,
  } = opts

  const sawOutageNeeded = requestFailed || oldPid === undefined
  let sawOutage = false
  let healthyPid: number | undefined
  let consecutiveSuccesses = 0

  // Give the old process enough time to exit before accepting a health
  // response as "the new process" (only when we have a PID to compare).
  if (!sawOutageNeeded) {
    await new Promise((resolve) => setTimeout(resolve, 2000))
  }

  for (let i = 0; i < maxRetries; i++) {
    let isDown = false
    let currentPid: number | undefined

    try {
      const res = await fetch(`/api/v1/health?t=${Date.now()}`, {
        method: 'GET',
        cache: 'no-store',
      })
      if (res.ok) {
        const body = await res.json()
        currentPid = Number(body?.data?.pid)
      } else {
        isDown = true
      }
    } catch {
      isDown = true
    }

    onProbe?.(isDown, currentPid, i + 1)

    if (isDown) {
      sawOutage = true
      healthyPid = undefined
      consecutiveSuccesses = 0
    } else if (Number.isFinite(currentPid)) {
      const isNewProcess = !oldPid || currentPid !== oldPid
      if (isNewProcess && (!sawOutageNeeded || sawOutage)) {
        if (healthyPid === currentPid) {
          consecutiveSuccesses += 1
        } else {
          healthyPid = currentPid
          consecutiveSuccesses = 1
        }

        // Require the same new process to stay healthy across two polls.
        if (consecutiveSuccesses >= 2) {
          return { recovered: true, newPid: currentPid, attempts: i + 1 }
        }
      }
    }

    await new Promise((resolve) => setTimeout(resolve, interval))
  }

  return { recovered: false, attempts: maxRetries }
}
