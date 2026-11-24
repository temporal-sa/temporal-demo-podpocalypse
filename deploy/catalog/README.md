# A real pod crash in the catalog

In the catalog, a wreck deletes a real Kubernetes pod: the shared session
worker pod, which its Deployment replaces. The game uses `RUNTIME=pod` for
this (see the [README](../../README.md#how-the-workflows-are-cut)).

The DemoProject grants the booth the pod access it needs with
`kubernetesAccess`. The operator renders it in every rollout's runtime
namespace, so there is no manual step.

## The DemoProject

In `temporal-sa/temporal-fieldkit-platform`, the `booth` and `sessions`
components of `projects/live/demo/podpocalypse.yaml` (and
`projects/test/demo/podpocalypse.yaml`) are:

```yaml
  components:
    booth:
      build: app
      worker: true
      temporalAccess: true
      # The booth deletes the session pod on a crash and runs the read-only
      # k9s panel: the operator renders its RBAC in every rollout's namespace.
      kubernetesAccess:
        observe: true
        crash: [sessions]
      env:
        - name: RUNTIME
          value: pod
        # PUBLIC_URL, CONTROL_TASK_QUEUE, EVENT_LEADS_ENDPOINT as before
      ports:
        - name: http
          containerPort: 8080
          servicePort: 80
    # Every flight's session worker. A crash deletes this pod.
    sessions:
      build: app
      worker: true
      # Fixed at one: autoscaling would add pods the crash does not delete.
      replicas: 1
      env:
        - name: ROLE
          value: session
        - name: TASK_QUEUE
          value: podpocalypse-sessions
```

`observe` lets the booth read pods, their logs, events and Deployments in its
runtime namespace, for the k9s panel. `crash: [sessions]` lets it delete the
`sessions` pods, and only those: the platform's admission policy refuses a
delete of any other component's pod. The booth's token is mounted, and API
egress is opened for its namespace. The platform's `docs/crd-reference.md`
(`kubernetesAccess`) has the full contract.

The sessions pod's labels are the operator's
(`app.kubernetes.io/component: sessions`), which is the booth's default
`SESSION_POD_SELECTOR`. If you rename the component, change `crash` and set
`SESSION_POD_SELECTOR` on the booth to match.

## Check it

Play a flight at https://podpocalypse.tmprl-demo.cloud and crash. The k9s
strip shows the `sessions-…` pod terminate and a new one start, and the Event
History strip says the new pod replayed the flight. From a terminal, find the
namespace the booth runs in now and watch it:

```bash
ns=$(kubectl --context <live-context> -n fieldkit-podpocalypse get svc booth \
  -o jsonpath='{.metadata.annotations.platform\.temporal\.io/active-namespace}')
kubectl --context <live-context> -n "$ns" get pods -w
```

## Undo

Set the booth's `RUNTIME` back to `process`, and drop `kubernetesAccess` and
the `sessions` component. The operator removes the booth's Role, RoleBinding
and token on the next reconcile.

## Try it locally

[`kind-runtime-namespace.yaml`](kind-runtime-namespace.yaml) lays out one
runtime namespace the way the operator does, with a dev server in place of
Temporal Cloud, and the booth's Role as the operator renders it:

```bash
kind create cluster --name podpocalypse
docker build -t podpocalypse:dev . && docker pull temporalio/temporal:1.5.1
kind load docker-image podpocalypse:dev temporalio/temporal:1.5.1 --name podpocalypse
kubectl --context kind-podpocalypse apply -f deploy/catalog/kind-runtime-namespace.yaml
kubectl --context kind-podpocalypse -n fieldkit-podpocalypse-abcde port-forward svc/booth 18090:80
```

Open http://localhost:18090 and fly. `kind create cluster` switches your
current kubectl context to the new cluster, so switch it back afterwards.
