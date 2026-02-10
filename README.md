# Kubernetes Workshop: From Ephemeral to Persistent

A hands-on workshop that demonstrates the difference between ephemeral pod storage and persistent database-backed storage on the Intility Developer Platform. You'll talk to Claude Code to set up infrastructure and deploy apps, then explore what it built using `oc`.

## What You'll Learn

- Using the **developer-platform-companion** skill to provision clusters, set up GitOps, and deploy applications
- How pods are ephemeral — data stored in memory dies with the pod
- Running a PostgreSQL database in Kubernetes using CloudNativePG
- Connecting an application to a database via environment variables
- How persistent storage survives pod restarts

## Prerequisites

- Access to the [Intility Developer Platform](https://developers.intility.com)
- [Install the `indev` CLI](https://article.intility.com/en-us/51ec0d96-220b-4e66-402b-08dc346b24fd#get-started-with-indev:install-indev)
- [Claude Code](https://claude.ai/code) installed

> **No Claude Code?** Follow the [manual guide](MANUAL_GUIDE.md) instead — same workshop, all `oc` commands.

## Repository Layout

```
k8s/
  app/            # The todo application
    app.yaml          ConfigMap, Deployment, Service, Route
    kustomization.yaml
  database/       # PostgreSQL via CloudNativePG
    cluster.yaml      Credentials Secret + CNPG Cluster
    kustomization.yaml
```

---

## Part 0: Get Started

Install the **developer-platform-companion** skill and start the onboarding — all in one go:

```bash
(claude plugin marketplace add git@github.com:intility/claude-plugins.git || true) && \
claude plugin install developer-platform-companion@intility-claude-plugins && \
claude "Help me onboard on the Intility Developer Platform. I have a todo app in k8s/app I want to deploy."
```

The skill will guide you through the entire setup — creating a cluster, logging in, installing ArgoCD, configuring GitOps credentials, and deploying the app. Just follow along and answer its questions.

When it's all done, verify you're connected:

```bash
oc whoami
oc get nodes
```

---

## Part 1: Explore the App (In-Memory)

The skill has deployed the todo app via ArgoCD. Take a look at what got created:

```bash
oc get pods,svc,route -l app=todo-app
```

| Resource | Purpose |
|----------|---------|
| **ConfigMap** `todo-config` | Environment variables for the app (empty for now) |
| **Deployment** `todo-app` | Runs the todo application |
| **Service** `todo-app` | Internal network endpoint |
| **Route** `todo-app` | Exposes the app externally (HTTPS) |

### 1.1 Open the App

```bash
oc get route todo-app -o jsonpath='https://{.spec.host}{"\n"}'
```

Open the URL in your browser. You should see the todo app with a **yellow banner**: **"Storage: In-Memory (ephemeral)"**.

### 1.2 Add Some Todos

Add a few items through the UI. They show up in the list — everything works.

### 1.3 Kill the Pod

```bash
oc delete pod -l app=todo-app
```

The Deployment immediately creates a replacement pod. Watch it come back:

```bash
oc get pods -l app=todo-app -w
```

### 1.4 Check Your Todos

Refresh the browser. **Your todos are gone.**

The new pod started fresh with an empty in-memory store. This is the key lesson: **pods are ephemeral**. Anything stored inside a pod is lost when it restarts. Pods can be killed at any time — during scaling, node maintenance, deployments, or crashes.

---

## Part 2: Add a PostgreSQL Database

Back in Claude Code:

> *"Deploy the PostgreSQL cluster I have in k8s/database"*

The skill creates another ArgoCD Application that syncs the CNPG cluster. Check what got created:

```bash
oc get cluster,pods -l cnpg.io/cluster=postgres
```

| Resource | Purpose |
|----------|---------|
| **Secret** `postgres-credentials` | Database username and password |
| **Cluster** `postgres` | A CloudNativePG PostgreSQL instance with 1Gi storage |

Wait for the database to be healthy:

```bash
oc get cluster postgres -w
```

When `STATUS` shows `Cluster in healthy state`, you're good.

### 2.1 Connect the App to PostgreSQL

Now tell the app where to find the database. Patch the ConfigMap and restart:

```bash
oc patch configmap todo-config -p '{"data":{"DATABASE_URL":"postgres://todos:todos@postgres-rw:5432/todos?sslmode=disable"}}'
oc rollout restart deployment/todo-app
oc rollout status deployment/todo-app
```

### 2.2 Verify the Connection

Refresh the browser. The banner should now be **green**: **"Storage: PostgreSQL (persistent)"**.

Check the logs:

```bash
oc logs -l app=todo-app
```

```
Using PostgreSQL store
Listening on :8080
```

### 2.3 Test Persistence

Add a few todos. Then kill the pod again:

```bash
oc delete pod -l app=todo-app
```

Wait for the new pod, refresh the browser. **Your todos are still there.**

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

- **Part 1**: Todos lived in a Go slice inside the process. Pod died, process died, data gone.
- **Part 2**: Todos live in PostgreSQL with a PersistentVolumeClaim. The app pod can die and restart freely — the data is safe in the database.

## How It Works

The app checks for a `DATABASE_URL` environment variable at startup:

- **Not set** → in-memory store (Go slice)
- **Set** → connects to PostgreSQL, auto-creates the `todos` table

The ConfigMap `todo-config` is injected as environment variables via `envFrom`. When you patched the ConfigMap and restarted the Deployment, the new pod started with `DATABASE_URL` set, so it connected to PostgreSQL instead.

## Cleanup

```bash
oc delete -k k8s/database
oc delete -k k8s/app
```
