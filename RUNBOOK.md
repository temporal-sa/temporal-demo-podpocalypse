# Runbook: Podpocalypse ("Kill the worker") demo

This runbook is for presenting the demo at a booth or on a call. The [README](README.md) explains the game and how the Workflows are cut.

## What it shows

A player flies Ziggy's pod-ship for 60 seconds. Every wreck deletes the Worker running that flight. The game freezes, the clock keeps running, and when a replacement Worker starts, Temporal replays the flight's Workflow and the player carries on with the same score and the same deadline. The time lost waiting is the penalty.

The demo makes two problems visible:

- **A process dies in the middle of someone's session, and its in-memory state goes with it.** Here the Worker that holds the flight is deleted on every crash. The replacement rebuilds the flight from the Workflow's Event History, so gates already cleared stay cleared. Beat 3 shows it.
- **A deadline has to keep counting while nothing is running.** The flight's clock is a durable Timer on the Temporal Service, not a timer in the Worker. It keeps running while the Worker is gone, so recovery time costs the player real seconds. Beats 3 and 4 show it.

It also shows one Workflow per player, keyed by a hash of their email, and one long-lived Workflow for the standings.

## Before you start

- **On tmprl-demo.cloud** you need only a browser. The booth is at https://podpocalypse.tmprl-demo.cloud and doesn't ask for a sign-in. It runs with `RUNTIME=process`: the booth container is the control Worker, and each flight's Worker is a child process it starts and kills. There is no Kubernetes pod per flight on tmprl-demo.cloud. Everyone who plays it shares the one leaderboard.
- Use a laptop or tablet in landscape for the booth screen. On a window 720 pixels wide or less, or 520 pixels tall or less, the page hides the Worker name in the bottom-left corner, which beat 3 points at.
- **To run it locally** you need [Go](https://go.dev/dl/) (the version in `go.mod`) and either the [Temporal CLI](https://docs.temporal.io/cli) or Docker for the dev server. No API keys.
- **To run each flight in its own Kubernetes pod**, use minikube; the README's [Minikube](README.md#minikube-still-with-the-dev-server) section covers it. `RUNTIME=kubernetes` creates a one-replica Deployment per flight and deletes the running pod on a crash.

## Start it

**On tmprl-demo.cloud:** open https://podpocalypse.tmprl-demo.cloud. A healthy start shows the **Kill the worker** form with **Email**, **Name**, **Title**, the consent box, and **Start flying**, over Ziggy drifting through the attract animation. The standings show on the right.

**Locally**, in two terminals from the repository root:

1. `temporal server start-dev` (or the README's `docker run` command for the dev server)
2. `TEMPORAL_UI_URL=http://localhost:8233 go run ./cmd/podpocalypse`

The booth logs `booth listening addr=:8080 runtime=process`. Open http://localhost:8080. `TEMPORAL_UI_URL` adds an **Open the workflow** link to the result screen. The Temporal UI is at http://localhost:8233.

For a standings-only screen next to the booth, open the same URL with `?board`.

## The walkthrough

A flight takes 60 seconds plus any recovery time. Run it once yourself before handing the booth to players.

### 1. The player signs in

1. The player fills in **Email**, **Name** and **Title**, ticks **You can reach me about the prize.** if they want, and presses **Start flying**.
2. A returning player only needs their email: the form shows "Welcome back, <name>. Email is enough to fly again."
3. The flight starts within a few seconds. The bottom-left corner shows the name of the Worker running it, `proc-<session>-1` on tmprl-demo.cloud (a pod name `sess-<session>-…` on Kubernetes).
4. Say: "That press started a Workflow for this flight, and a Worker that serves only this flight. The booth itself is a different Worker, and it's the one we never kill."

### 2. Fly

1. Tap, click, or press `Space` or `↑` to climb. Each column cleared is a gate; the score is top left and the clock top right.
2. Each gate goes to the flight's Workflow as an Update. Flying through a treat shows `+8s` and adds 8 seconds to the deadline, up to four times per flight.
3. Say: "Every gate is recorded in the Workflow, not in the browser. The Workflow checks each one and rejects a skipped or impossibly fast gate."

### 3. Crash, and the Worker is deleted

1. Hit a column, the ceiling or floor, an `OOMKilled` / `CrashLoopBackOff` / `Evicted` / `ImagePullBackOff` block, or the rolling restart wave.
2. The screen shows **SHIP DESTROYED** with the cause and "Waiting for a new worker process… the clock is still running." (on Kubernetes: "Waiting for the Kubernetes scheduler…"). The ship freezes and the clock keeps counting down.
3. Behind it, the Workflow schedules an Activity on the booth's control Worker that kills the flight's Worker and waits until it's gone. The replacement starts 4 seconds later (`RECOVERY_DELAY`), replays the Workflow, and the flight resumes with the same score. The Worker name in the corner changes from `-1` to `-2`.
4. Say: "The process holding this game was just killed. The new one has nothing in memory. It replays the Workflow's history, gets back the score and the deadline, and carries on. The clock never stopped, because the timer lives in Temporal, not in the Worker."
5. For proof, open `session-<session>` in the Temporal UI (the session ID is in the Worker name). In its history, after the crash's `WorkflowExecutionUpdateAccepted` and the `DeleteSessionPod` Activity, a `WorkflowTaskScheduled` waits several seconds for its `WorkflowTaskStarted`: that gap is the time with no Worker. Then the `IdentifyWorker` Activity runs on the new Worker and the Update completes.

If the player picked up a **STANDBY** before crashing, the screen shows "Standby pod deploying" and the flight carries on after a moment without deleting anything. That perk lives only in the browser; crash again without one to show the real deletion.

### 4. The flight ends, and the standings update

1. When the clock runs out, the result screen shows the score, "You cleared N gates with M crashes.", and "Best score yet!" or "Your best: N". Locally, **Open the workflow** links to the flight's Workflow.
2. **Standings** lists the top 10, one best entry per player. "Next flight in 8" counts down and the form comes back for the next player.
3. Say: "Each player is a Workflow keyed by their email, and the standings are one long-running Workflow. Nothing here has a database."
4. For proof, show `leaderboard`, `run-<session>` and `player-<hash>` in the Temporal UI.

### 5. Optional: event booths and flight codes

On tmprl-demo.cloud the booth is connected to the event lead tracker through a Nexus Endpoint (`event-leads-participants`). A Temporal employee signed in to tmprl-demo.cloud sees an **Event** list and **Open this event** on the form. An event page, `/event/<id>`, shows a QR code; players fill in the form on their phone and type the 4-digit **Flight code** on the booth instead of their details. Each lookup is a short Workflow that calls a Nexus Operation on the tracker's service.

## Show it in code

All links point at commit [`893d501`](https://github.com/temporal-sa/temporal-demo-podpocalypse/commit/893d501d4830a7d102da02726c6662b9f3b3f853). Open a file locally with `code -g <path>:<line>`.

### Delete the Worker mid-game and resume where it stopped

**When to use it:** after beat 3, or when someone asks how the score survived.

**Where it is:** [`internal/workflows/session.go` lines 245–285](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/workflows/session.go#L245-L285) (`code -g internal/workflows/session.go:245`)

```go
func (s *sessionState) crash(ctx workflow.Context) (CrashResult, error) {
	s.busy = true
	// ...
	s.phase = "recovering"
	// ...
	// Recorded on the control queue before the session pod is deleted, and
	// completed only after that pod is gone, so the replacement resumes here.
	killCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           s.in.ControlTaskQueue,
		StartToCloseTimeout: 2 * time.Minute,
		HeartbeatTimeout:    20 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 5,
			InitialInterval: time.Second,
		},
	})
	var acts *infra.Infra
	if err := workflow.ExecuteActivity(killCtx, acts.DeleteSessionPod, s.in.SessionID).Get(ctx, nil); err != nil {
		return CrashResult{}, err
	}
	s.recoveryTotal += workflow.Now(ctx).Sub(started)
	// ...
	s.crashes++
	if err := s.identify(ctx); err != nil {
		return CrashResult{}, err
	}
	// ...
	s.phase = "playing"
```

**What you need to understand:**

- The crash is an Update handler on the flight's Workflow. Once the Update is accepted it's in the Event History, so the handler survives the death of the Worker that was running it.
- The kill runs as an Activity on the booth's control Task Queue, not on the flight's own queue. The Worker being deleted can't be the one doing the deleting.
- When the kill Activity completes, the next Workflow Task has nowhere to go until the replacement Worker polls the flight's Task Queue. The replacement has no cached state, so it replays the history from the start: every gate, the crash, the kill. It ends up in this handler, right after the Activity, with the same `score`.
- `identify` then runs a small Activity on the flight's own queue ([lines 287–302](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/workflows/session.go#L287-L302)), with a 3-minute `ScheduleToStartTimeout`. That records which Worker came back, which is the name in the corner of the screen.
- While no Worker is up, the browser's Query for the flight's state fails, and the API answers "recovering" ([`internal/api/server.go` lines 278–286](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/api/server.go#L278-L286)). A Query needs a live Worker; the history doesn't.
- The customer doesn't write checkpointing or state recovery. The Workflow code is written as if the process never dies.

**Docs:** [Workflow Execution and replay](https://docs.temporal.io/workflow-execution), [Go: Update handlers](https://docs.temporal.io/develop/go/workflows/message-passing#updates)

### Kill the Worker exactly once, even when the Activity retries

**When to use it:** when someone asks what stops a retry from killing the replacement.

**Where it is:** [`internal/infra/activities.go` lines 127–181](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/infra/activities.go#L127-L181) (`code -g internal/infra/activities.go:127`)

```go
func (a *Infra) DeleteSessionPod(ctx context.Context, sessionID string) error {
	var marker killMarker
	if activity.IsActivity(ctx) && activity.HasHeartbeatDetails(ctx) {
		if err := activity.GetHeartbeatDetails(ctx, &marker); err != nil {
			return err
		}
	}
	if a.Runtime == config.RuntimeKubernetes {
		return a.killPod(ctx, sessionID, marker)
	}
	return a.killProcess(ctx, sessionID, marker)
}
// ...
func (a *Infra) killPod(ctx context.Context, sessionID string, marker killMarker) error {
	if marker.UID == "" {
		uid, err := a.currentUID(ctx, sessionID)
		// ...
		marker.UID = uid
		if activity.IsActivity(ctx) {
			activity.RecordHeartbeat(ctx, marker)
		}
	}
	exists, err := a.uidExists(ctx, sessionID, marker.UID)
	// ...
	if !exists {
		return nil
	}
	// ...
	return a.waitUIDGone(ctx, sessionID, marker)
}
```

**What you need to understand:**

- The first attempt picks the pod UID (or process ID) to kill and records it in a heartbeat before doing anything. If the attempt fails, the retry receives that heartbeat and targets the same generation.
- If that generation is already gone, the retry returns success. A Deployment's replacement pod has a different UID, so it's never touched; the pod delete also carries a UID precondition.
- The Activity returns only after the old Worker has exited. The flight's next Workflow Task isn't scheduled until then, so the replacement can't pick up the flight while the old Worker is still alive.
- `HeartbeatTimeout: 20 * time.Second` in the Workflow means a booth that dies mid-kill is noticed in 20 seconds and the attempt is retried, up to 5 attempts.
- The pattern is general: record what you're about to act on, then make each attempt check for it. It's how a customer makes any non-idempotent Activity safe to retry.

**Docs:** [Activity heartbeats](https://docs.temporal.io/encyclopedia/detecting-activity-failures#activity-heartbeat), [Go: heartbeat details](https://docs.temporal.io/develop/go/activities/timeouts#activity-heartbeats)

### Keep the clock running while no Worker is up

**When to use it:** during beat 3, when someone notices the clock didn't stop.

**Where it is:** [`internal/workflows/session.go` lines 156–191](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/workflows/session.go#L156-L191) (`code -g internal/workflows/session.go:156`)

```go
timerFired := false
timer := workflow.NewTimer(ctx, in.Duration)
for !s.finished {
	sel := workflow.NewSelector(ctx)
	sel.AddReceive(s.wake, func(c workflow.ReceiveChannel, _ bool) {
		// ...
	})
	// ...
	if !timerFired {
		sel.AddFuture(timer, func(workflow.Future) { timerFired = true })
	}
	sel.Select(ctx)
	// ...
	// The clock is a durable timer. It keeps running while this session's
	// worker is gone; the flight only ends once a worker is back and the
	// crash handler has finished deleting the pod.
	if timerFired && !s.busy {
		now := workflow.Now(ctx)
		// A treat pushes the deadline past the timer that was already
		// scheduled. Start another timer for whatever time was added.
		if now.Before(s.deadline) {
			timerFired = false
			timer = workflow.NewTimer(ctx, s.deadline.Sub(now))
			continue
		}
		s.finish(ctx)
	}
}
```

**What you need to understand:**

- `workflow.NewTimer` creates a Timer on the Temporal Service, recorded as `TimerStarted` in the history. It fires on time whether or not a Worker is running; the Workflow sees it at its next Workflow Task.
- The deadline is set once from `workflow.Now` when the flight starts and is returned in every Query, so the browser draws the clock from the Workflow's deadline, not its own.
- A treat moves the deadline; when the first Timer fires early, the loop starts a new one for the time that was added.
- If the Timer fires mid-crash, `s.busy` holds the finish until the kill completes. A flight never ends while its Worker is being replaced.
- A customer uses the same thing for an SLA, a reservation hold, or a payment window that has to expire on time even if every Worker is down.

**Docs:** [Timers](https://docs.temporal.io/develop/go/timers)

### Reject impossible moves before they reach the history

**When to use it:** during beat 2, or when someone asks whether a player could post a fake score.

**Where it is:** [`internal/workflows/session.go` lines 118–127](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/workflows/session.go#L118-L127) and [lines 195–206](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/workflows/session.go#L195-L206) (`code -g internal/workflows/session.go:195`)

```go
if err := workflow.SetUpdateHandlerWithOptions(ctx, UpdateGate,
	func(ctx workflow.Context, gate int) (GateResult, error) {
		return s.passGate(ctx, gate)
	},
	workflow.UpdateHandlerOptions{Validator: func(ctx workflow.Context, gate int) error {
		return s.validateGate(ctx, gate)
	}},
); err != nil {
// ...
func (s *sessionState) validateGate(ctx workflow.Context, gate int) error {
	if s.finished || s.phase != "playing" {
		return fmt.Errorf("not flying (%s)", s.phase)
	}
	if gate != s.score+1 {
		return fmt.Errorf("gate %d is not the next gate", gate)
	}
	if !s.lastGate.IsZero() && workflow.Now(ctx).Sub(s.lastGate) < game.MinGateInterval {
		return fmt.Errorf("gate %d came in too fast", gate)
	}
	return nil
}
```

**What you need to understand:**

- A gate is accepted only if it's the next one, at least 280 ms after the last (`game.MinGateInterval`), and only while the flight is playing. Gates sent during a recovery are refused.
- A rejected Update writes nothing to the Event History. The caller gets the error back. On Temporal Cloud a rejected Update still counts as an Action.
- The game physics run in the browser; the Workflow is the referee. Customers use validators the same way, for example to refuse a change to an order that has already shipped.

**Docs:** [Go: Update handlers and validators](https://docs.temporal.io/develop/go/workflows/message-passing#updates)

### Give each flight its own Worker, and always clean it up

**When to use it:** when someone asks how the per-flight Worker is created and removed.

**Where it is:** [`internal/workflows/run.go` lines 40–73](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/workflows/run.go#L40-L73) (`code -g internal/workflows/run.go:40`)

```go
// Teardown has to run even when the game is cancelled, and it has to be
// recorded in history in the same place on replay. Disconnected context
// keeps the activity from inheriting that cancellation.
defer func() {
	dctx, cancel := workflow.NewDisconnectedContext(ctx)
	defer cancel()
	// ...
	if err := workflow.ExecuteActivity(dctx, acts.TeardownSessionWorker, in.SessionID).Get(dctx, nil); err != nil {
		workflow.GetLogger(ctx).Error("session teardown failed", "error", err)
	}
}()

if err := workflow.ExecuteActivity(actx, acts.ProvisionSessionWorker, infra.ProvisionInput{SessionID: in.SessionID}).Get(ctx, nil); err != nil {
	return RunResult{}, err
}

childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
	WorkflowID:               names.SessionWorkflowID(in.SessionID),
	TaskQueue:                names.SessionQueue(in.SessionID),
	WorkflowExecutionTimeout: in.Duration + 5*time.Minute,
	ParentClosePolicy:        enumspb.PARENT_CLOSE_POLICY_TERMINATE,
})
```

**What you need to understand:**

- `GameRunWorkflow` runs on the booth's control Worker. It provisions the flight's Worker, which polls a Task Queue named `session-<id>`, then starts the flight as a Child Workflow on that queue. Only that one Worker can run the flight.
- `ProvisionSessionWorker` returns only once the new Worker is polling, so the Child Workflow never waits for a Worker that hasn't started.
- Teardown runs in a disconnected context, so it still runs if the run is cancelled. `ParentClosePolicy: TERMINATE` ends the flight if its parent closes, and the 5-minute margin on the execution timeout caps a flight whose Worker never comes back.
- After the flight, the parent signals the standings and the player, and appends the lead ([lines 78–118](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/workflows/run.go#L78-L118)).
- A Worker per flight is a demo device: it makes "kill the Worker" mean exactly one game. A customer runs a shared Worker pool, and the recovery works the same way.

### One Workflow per player, one for the standings

**When to use it:** during beat 4, or when someone asks where player data is stored.

**Where it is:** [`internal/api/server.go` lines 491–514](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/api/server.go#L491-L514) (`code -g internal/api/server.go:491`), with the Workflows in [`internal/workflows/entities.go` lines 65–99](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/workflows/entities.go#L65-L99) (standings) and [lines 150–220](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/workflows/entities.go#L150-L220) (player)

```go
func (s *Server) ensurePlayer(ctx context.Context, in workflows.PlayerInput) (workflows.PlayerState, error) {
	_, err := s.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    in.PlayerID,
		TaskQueue:             s.Config.ControlQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
	}, workflows.PlayerWorkflow, in)
	if err != nil && !temporal.IsWorkflowExecutionAlreadyStartedError(err) {
		return workflows.PlayerState{}, err
	}
	handle, err := s.Client.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   in.PlayerID,
		UpdateName:   workflows.UpdateProfile,
		Args:         []any{in},
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})
	// ...
}
```

**What you need to understand:**

- The player's Workflow ID is `player-` plus a SHA-256 prefix of the lower-cased email ([`internal/names/names.go` lines 52–56](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/names/names.go#L52-L56)), so the same email always reaches the same Workflow and the address isn't in the ID.
- Starting it again returns "already started", which the code treats as success, then an Update refreshes the name, title and consent. The reuse policy allows a new run only if the previous one failed, was terminated, cancelled, or timed out.
- The player Workflow keeps games played and a single best score, updated by a Signal after each flight. The `leaderboard` Workflow keeps one best entry per player, answers the standings Query, and continues-as-new after 200 scores or when Temporal suggests it.
- Workflow IDs are unique per Namespace among running Workflows, so there's no race between two booths creating the same player. This is the entity Workflow pattern.

**Docs:** [Entity Workflow pattern](https://docs.temporal.io/design-patterns/entity-workflow), [Continue-As-New](https://docs.temporal.io/workflow-execution/continue-as-new)

### Look up event leads through Nexus

**When to use it:** during beat 5, or when someone asks how the booth talks to another team's service.

**Where it is:** [`internal/workflows/leads.go` lines 90–105](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/workflows/leads.go#L90-L105) (`code -g internal/workflows/leads.go:90`)

```go
func ClaimFlight(ctx workflow.Context, in ClaimFlightInput) (FlightProfile, error) {
	var profile FlightProfile
	err := callLeads(ctx, in.Endpoint, "claim_flight_pass", map[string]string{
		"eventId": in.EventID,
		"code":    in.Code,
	}, &profile)
	return profile, err
}

func callLeads(ctx workflow.Context, endpoint, operation string, input, output any) error {
	if endpoint == "" {
		return temporal.NewNonRetryableApplicationError("event leads are not configured", "Unconfigured", nil)
	}
	future := workflow.NewNexusClient(endpoint, LeadServiceName).ExecuteOperation(ctx, operation, input, workflow.NexusOperationOptions{})
	return future.Get(ctx, output)
}
```

**What you need to understand:**

- The booth never calls the lead tracker over HTTP. A short Workflow calls a Nexus Operation on the `event-leads` service through the Endpoint named in `EVENT_LEADS_ENDPOINT`, and Temporal routes it to the tracker's Namespace and Workers.
- The booth knows only the Endpoint name, the service name and the operation names. The tracker's Namespace, Task Queue and code stay behind the Endpoint.
- Unset `EVENT_LEADS_ENDPOINT`, as in a local run, and the event and flight-code features are off; the open booth works without them.
- The tracker side isn't in this repo.

**Docs:** [Temporal Nexus](https://docs.temporal.io/nexus), [Go: Nexus](https://docs.temporal.io/develop/go/nexus)

### What's demo-only

When someone asks what they'd change for their own system, these are the parts to leave out:

- **Killing your own Workers.** `DeleteSessionPod` and the process supervisor exist to cause the failure. A real system gets the failure for free.
- **The process supervisor.** [`internal/infra/process.go` lines 16–19](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/infra/process.go#L16-L19): with `RUNTIME=process` (local and tmprl-demo.cloud) it stands in for the Kubernetes scheduler, restarting a flight's Worker `RECOVERY_DELAY` after it exits.
- **The recovery hold on Kubernetes.** [`internal/infra/activities.go` lines 183–204](https://github.com/temporal-sa/temporal-demo-podpocalypse/blob/893d501d4830a7d102da02726c6662b9f3b3f853/internal/infra/activities.go#L183-L204) keeps the kill Activity open for `RECOVERY_DELAY` after the pod is gone, so the penalty is visible even when the replacement pod is already up.
- **A Worker and Task Queue per flight.** See [Give each flight its own Worker](#give-each-flight-its-own-worker-and-always-clean-it-up).
- **The game itself:** the physics, hazards, treats and the STANDBY perk are browser code. Only gates, treats and crashes reach the Workflow.

## Reset and tear down

- **Between players:** nothing to do. The result screen returns to the form after 8 seconds, and each flight's Worker is torn down when its run ends.
- **To clear the standings locally:** terminate the `leaderboard` Workflow, then restart the booth, which starts a fresh one at startup:

  ```bash
  temporal workflow terminate -w leaderboard --reason "reset standings"
  ```

  Don't do this on tmprl-demo.cloud; that leaderboard is shared. Player Workflows keep running and keep each player's best score; a local dev server forgets them when it stops.
- **To stop locally:** `Ctrl-C` the booth, which stops any flight Workers it started, then stop the dev server (`Ctrl-C`, or `docker rm -f podpocalypse-temporal`). Finished games are in the file named by `LEAD_PATH`, if you set one.
- **On Kubernetes:** the booth deletes flight Deployments older than 20 minutes. `kubectl delete namespace podpocalypse` removes everything.
- **On tmprl-demo.cloud:** leave it running. The booth starts Workflows only when someone presses **Start flying**; the leaderboard, kiosk and player Workflows wait without running anything.

## When something goes wrong

| What you see | Likely cause | Fix |
|---|---|---|
| The booth exits at startup with `control worker did not poll` | It couldn't reach Temporal within 30 seconds | Start the dev server first; check `TEMPORAL_ADDRESS` |
| "could not open your player record" | The booth can't reach Temporal, or its control Worker isn't running | Check `GET /healthz`, which returns "temporal is not reachable" when the connection is down |
| "the session worker did not start" | The flight's Worker didn't poll within 25 seconds: the process couldn't start, or on Kubernetes the pod wasn't scheduled or the image didn't pull | Check the booth log; on Kubernetes, `kubectl -n podpocalypse get pods` |
| A crash shows "Standby pod deploying" and nothing is deleted | The player had a STANDBY perk | Expected; crash again without one |
| No Worker name in the bottom-left corner | The window is 720 pixels wide or less, or 520 pixels tall or less | Use a larger window, or show the name in the Temporal UI |
| No **Open the workflow** link on the result screen | `TEMPORAL_UI_URL` isn't set; the tmprl-demo.cloud catalog entry doesn't set it | Open the Namespace in the Temporal UI and search for `session-<session>` |
| Standings stay empty after finished flights | The `leaderboard` Workflow isn't running | Restart the booth; it starts the leaderboard at startup |
| "that flight code is not valid" or "that flight code was already used" | A wrong, expired or reused code on an event page | Have the player finish the form on their phone again |
| "this booth is not connected to the event tracker" | `EVENT_LEADS_ENDPOINT` isn't set | Expected locally; use the open booth form |

## Keeping this runbook current

The code links point at a fixed commit. When a linked file changes, update its link, line numbers, and snippet together. Before an event, list the linked files that have changed:

```sh
grep -oE 'blob/[0-9a-f]{40}/[^#)]+' RUNBOOK.md | sort -u | while IFS=/ read -r _ sha file; do
  git diff --quiet "$sha" HEAD -- "$file" || echo "changed since ${sha:0:7}: $file"
done
```
