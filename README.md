# Kubernetes Workshop: From Ephemeral to Persistent

A hands-on workshop that demonstrates the difference between ephemeral pod storage and persistent database-backed storage in Kubernetes. You'll deploy a simple todo app, watch your data disappear when a pod dies, then fix it with a PostgreSQL database.

## What You'll Learn

- Deploying an application with Kustomize
- How pods are ephemeral — data stored in memory dies with the pod
- Running a PostgreSQL database in Kubernetes using CloudNativePG
- Connecting an application to a database via environment variables
- How persistent storage survives pod restarts

## Prerequisites

- Access to an OpenShift cluster (you should already be logged in with `oc`)
- The `kubectl` or `oc` CLI
- CloudNativePG operator installed on the cluster (ask your instructor)

## Repository Layout

```
k8s/
  app/            # Part 1 — the todo application
    app.yaml          ConfigMap, Deployment, Service, Route
    kustomization.yaml
  database/       # Part 2 — PostgreSQL via CloudNativePG
    cluster.yaml      Credentials Secret + CNPG Cluster
    kustomization.yaml
```

---

## Part 1: Deploy the App (In-Memory)

### 1.1 Deploy

Apply the app manifests:

```bash
kubectl apply -k k8s/app
```

This creates four resources:

| Resource | Purpose |
|----------|---------|
| **ConfigMap** `todo-config` | Environment variables for the app (empty for now) |
| **Deployment** `todo-app` | Runs the todo application |
| **Service** `todo-app` | Internal network endpoint |
| **Route** `todo-app` | Exposes the app externally (HTTPS) |

### 1.2 Verify It's Running

```bash
kubectl get pods -l app=todo-app
```

You should see one pod in `Running` state.

Get the URL of your app:

```bash
kubectl get route todo-app -o jsonpath='{.spec.host}'
```

Open it in your browser. You should see the todo app with a **yellow banner** that says **"Storage: In-Memory (ephemeral)"**.

### 1.3 Add Some Todos

Add a few todo items through the UI. They'll appear in the list — everything works.

### 1.4 Kill the Pod

Now delete the pod:

```bash
kubectl delete pod -l app=todo-app
```

Kubernetes will immediately create a new pod (that's what the Deployment does). Wait for it:

```bash
kubectl get pods -l app=todo-app -w
```

### 1.5 Check Your Todos

Refresh the browser. **Your todos are gone.** The new pod started fresh with an empty in-memory store.

This is the key lesson: **pods are ephemeral**. Anything stored inside a pod is lost when it restarts. In production, pods can be killed at any time — during scaling, node maintenance, deployments, or crashes.

---

## Part 2: Add a PostgreSQL Database

### 2.1 Deploy the Database

Apply the database manifests:

```bash
kubectl apply -k k8s/database
```

This creates:

| Resource | Purpose |
|----------|---------|
| **Secret** `postgres-credentials` | Database username and password |
| **Cluster** `postgres` | A CloudNativePG PostgreSQL instance with 1Gi storage |

Wait for the database to be ready:

```bash
kubectl get cluster postgres -w
```

When the `STATUS` column shows `Cluster in healthy state`, you're good. This may take a minute or two.

### 2.2 Connect the App to PostgreSQL

Tell the app where to find the database by patching the ConfigMap:

```bash
kubectl patch configmap todo-config -p '{"data":{"DATABASE_URL":"postgres://todos:todos@postgres-rw:5432/todos?sslmode=disable"}}'
```

Then restart the app so it picks up the new config:

```bash
kubectl rollout restart deployment/todo-app
kubectl rollout status deployment/todo-app
```

### 2.3 Verify the Connection

Refresh the browser. The banner should now be **green** and say **"Storage: PostgreSQL (persistent)"**.

Check the pod logs to confirm:

```bash
kubectl logs -l app=todo-app
```

You should see:

```
Using PostgreSQL store
Listening on :8080
```

### 2.4 Test Persistence

Add a few todos through the UI. Now delete the pod again:

```bash
kubectl delete pod -l app=todo-app
```

Wait for the new pod to come up, then refresh the browser. **Your todos are still there.** The data lives in PostgreSQL, not in the pod.

---

## What Just Happened?

```
Part 1                          Part 2

┌──────────┐                    ┌──────────┐       ┌────────────┐
│ todo-app │                    │ todo-app │──────▶│ PostgreSQL │
│  (pod)   │                    │  (pod)   │       │  (CNPG)    │
│          │                    │          │       │            │
│ [todos]  │ ◀── in memory,    │          │       │  [todos]   │ ◀── on disk,
│          │     dies with pod  │          │       │            │     survives
└──────────┘                    └──────────┘       └────────────┘
```

- **Part 1**: The app stored todos in a Go slice inside the process. When the pod died, the process died, and the data was gone.
- **Part 2**: The app stores todos in PostgreSQL, which runs in its own pod with a PersistentVolumeClaim. The todo-app pod can die and restart freely — the data is safe in the database.

## How It Works

The app checks for a `DATABASE_URL` environment variable at startup:

- **Not set** → uses an in-memory store (Go slice)
- **Set** → connects to PostgreSQL, auto-creates the `todos` table

The ConfigMap `todo-config` is injected into the pod as environment variables via `envFrom`. When you patched the ConfigMap and restarted the Deployment, the new pod started with `DATABASE_URL` set, so it connected to PostgreSQL.

## Cleanup

Remove everything when you're done:

```bash
kubectl delete -k k8s/database
kubectl delete -k k8s/app
```
