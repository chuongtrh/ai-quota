# Background Efficiency Design

AI Quota should remain responsive while minimizing work performed by a menu-bar application that runs all day. The current refresh interval remains 60 seconds so quota freshness does not regress. Instead, the UI clock moves from two seconds to 30 seconds because all displayed relative times have minute-level precision. Claude connection detection is cached for 60 seconds; enable and disable actions bypass the cache so configuration changes initiated by the application remain immediate.

Provider fetches run concurrently under their existing individual 15-second deadlines. Results are still applied through the service mutex, and the existing atomic refresh guard continues to prevent overlapping refresh cycles. This bounds total refresh latency by the slowest provider rather than the sum of all provider latencies.

The Codex executable path is resolved once and reused. If starting the cached executable fails because it disappeared or became invalid, the cached value is cleared so the next refresh performs normal discovery again. The App Server remains one-process-per-refresh for now; keeping a persistent process would require a larger lifecycle, reconnect, and protocol-error design.

Tests use controllable fake providers to prove parallel starts, a fake clock/checker to prove Claude settings reads are throttled and forceable, and a temporary executable to prove Codex path reuse and invalidation behavior. Full unit, vet, and race suites validate integration.
