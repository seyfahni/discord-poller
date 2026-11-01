# Implementation Plan

Implement the approved spec in GitHub issue #6 through its dependency-ordered tickets:

1. **#7 — Deployable Go bot service**: choose a maintained Go Discord library, implement MySQL storage behind a small storage interface, establish the bot application module and fake Discord adapter, add configuration, container support, and operator instructions.
2. **#8 — Server onboarding**: let each server administrator select its existing group-creation role and persist the setting in that server's scope.
3. **#9 — Game groups**: add slash-command group creation, private channel provisioning, schedule/time-zone/poll settings, organizer management, roster management, and channel-access synchronization.
4. **#10 — Recurring polls**: schedule weekly/fortnightly sessions in local time, create and close native polls, enforce Discord's 32-day limit, and recover scheduled work after restarts.
5. **#12 — Skip and reschedule**: implement the agreed notices, old-poll closure, replacement-poll timing, and fresh-vote behavior.
6. **#11 — Cleanup**: remove orphaned group data and scheduled work, end any open poll with a notice, preserve its channel/messages, and purge a server's data when the bot is removed.

Work test-first at the bot application module's interface. Use fake Discord interactions/actions, isolated storage, and a controllable clock for behavior tests; run the repository's Go tests, race detector, vet, and build checks after the slices.

The deliverable is a deployable service and setup instructions, not a live cloud deployment. No web dashboard, automated reminders, custom attendance buttons, unanswered-member summary, or bot-side attendance archive is included.

## Storage decision

Use MySQL for the hosted service's durable groups, schedules, and pending work. Keep storage behind an application-facing interface so another implementation can be added later; use an in-memory test adapter to exercise the same interface. The seam adds some abstraction and requires adapters to preserve the same behavior. The hosting provider remains undecided.
