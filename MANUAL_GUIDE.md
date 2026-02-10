# Kubernetes Workshop: Manual Guide

Step-by-step guide for the workshop without Claude Code. You'll set up everything manually using `indev`, `oc`, and ArgoCD.

## Prerequisites

- Access to the [Intility Developer Platform](https://developers.intility.com)
- [Install the `indev` CLI](https://article.intility.com/en-us/51ec0d96-220b-4e66-402b-08dc346b24fd#get-started-with-indev:install-indev)
- `oc` CLI installed ([download from your cluster's console](https://docs.openshift.com/container-platform/latest/cli_reference/openshift_cli/getting-started-cli.html))
- A GitHub account with access to this repository

---

## Part 0: Set Up Your Cluster

### 0.1 Create a Cluster

```bash
indev cluster create --name k8s-workshop --preset minimal --nodes 2
```

The platform appends a random suffix to the name (e.g. `k8s-workshop-abc123`). Note the full name from the output.

### 0.2 Wait for the Cluster

Poll until the status is `Ready`:

```bash
indev cluster status k8s-workshop-<suffix>
```

This takes a few minutes.

### 0.3 Log In

```bash
indev cluster login k8s-workshop-<suffix>
```

A browser window opens for authentication. After logging in, verify:

```bash
oc whoami
oc get nodes
```

### 0.4 Install the OpenShift GitOps Operator

```bash
oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: openshift-gitops-operator
  namespace: openshift-operators
spec:
  channel: latest
  name: openshift-gitops-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF
```

Wait for it to install:

```bash
oc get csv -n openshift-operators -w | grep gitops
```

It's ready when the phase shows `Succeeded`.

### 0.5 Configure ArgoCD RBAC

Allow ArgoCD to manage resources across namespaces:

```bash
oc adm policy add-cluster-role-to-user cluster-admin \
  -z openshift-gitops-argocd-application-controller \
  -n openshift-gitops
```

Add yourself as an ArgoCD admin:

```bash
USER_UPN=$(oc whoami)

oc patch argocd openshift-gitops -n openshift-gitops --type=merge -p "
spec:
  rbac:
    policy: |
      g, ${USER_UPN}, role:admin
    scopes: '[groups, email]'
"
```

### 0.6 Configure GitHub App Credentials

ArgoCD needs access to pull from your GitHub repositories.

1. Go to your GitHub Organization Settings > Developer settings > GitHub Apps > **New GitHub App**
2. Configure:
   - **Name**: `argocd-k8s-workshop`
   - **Homepage URL**: any valid URL
   - **Webhook**: uncheck "Active"
   - **Permissions**: Repository Contents (Read-only), Metadata (Read-only)
   - **Installation**: Only on this account
3. After creation, note the **App ID**
4. Go to Install App, install it, and note the **Installation ID** from the URL
5. Generate and download a **private key** (.pem file)

Create the credentials secret:

```bash
oc create secret generic github-app-creds \
  -n openshift-gitops \
  --from-literal=githubAppID=<APP_ID> \
  --from-literal=githubAppInstallationID=<INSTALLATION_ID> \
  --from-file=githubAppPrivateKey=<PATH_TO_KEY.pem>

oc label secret github-app-creds -n openshift-gitops \
  argocd.argoproj.io/secret-type=repo-creds
```

Create the credential template for your org:

```bash
oc apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: intility-repo-creds
  namespace: openshift-gitops
  labels:
    argocd.argoproj.io/secret-type: repo-creds
type: Opaque
stringData:
  type: git
  url: https://github.com/intility
  githubAppID: "<APP_ID>"
  githubAppInstallationID: "<INSTALLATION_ID>"
  githubAppPrivateKey: |
    <paste private key content here>
EOF
```

### 0.7 Install the CloudNativePG Operator

```bash
oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: cloudnative-pg
  namespace: openshift-operators
spec:
  channel: stable
  name: cloudnative-pg
  source: certified-operators
  sourceNamespace: openshift-marketplace
EOF
```

Wait for it:

```bash
oc get csv -n openshift-operators -w | grep cnpg
```

### 0.8 Create a Project

```bash
oc new-project todo-workshop
```

---

## Part 1: Deploy the App (In-Memory)

### 1.1 Create an ArgoCD Application

Get the ArgoCD URL:

```bash
oc get route openshift-gitops-server -n openshift-gitops -o jsonpath='https://{.spec.host}{"\n"}'
```

Create the ArgoCD Application for the todo app:

```bash
oc apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: todo-app
  namespace: openshift-gitops
spec:
  project: default
  source:
    repoURL: https://github.com/intility/k8sbasics
    targetRevision: HEAD
    path: k8s/app
  destination:
    server: https://kubernetes.default.svc
    namespace: todo-workshop
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
EOF
```

Wait for the app to sync:

```bash
oc get application todo-app -n openshift-gitops -w
```

It's ready when `SYNC STATUS` shows `Synced` and `HEALTH STATUS` shows `Healthy`.

### 1.2 Verify the Deployment

```bash
oc get pods,svc,route -l app=todo-app -n todo-workshop
```

| Resource | Purpose |
|----------|---------|
| **ConfigMap** `todo-config` | Environment variables for the app (empty for now) |
| **Deployment** `todo-app` | Runs the todo application |
| **Service** `todo-app` | Internal network endpoint |
| **Route** `todo-app` | Exposes the app externally (HTTPS) |

### 1.3 Open the App

```bash
oc get route todo-app -n todo-workshop -o jsonpath='https://{.spec.host}{"\n"}'
```

Open the URL in your browser. You should see the todo app with a **yellow banner**: **"Storage: In-Memory (ephemeral)"**.

### 1.4 Add Some Todos

Add a few items through the UI. They show up in the list — everything works.

### 1.5 Kill the Pod

```bash
oc delete pod -l app=todo-app -n todo-workshop
```

The Deployment immediately creates a replacement pod. Watch it come back:

```bash
oc get pods -l app=todo-app -n todo-workshop -w
```

### 1.6 Check Your Todos

Refresh the browser. **Your todos are gone.**

The new pod started fresh with an empty in-memory store. This is the key lesson: **pods are ephemeral**. Anything stored inside a pod is lost when it restarts. Pods can be killed at any time — during scaling, node maintenance, deployments, or crashes.

---

## Part 2: Add a PostgreSQL Database

### 2.1 Create an ArgoCD Application for PostgreSQL

```bash
oc apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: todo-database
  namespace: openshift-gitops
spec:
  project: default
  source:
    repoURL: https://github.com/intility/k8sbasics
    targetRevision: HEAD
    path: k8s/database
  destination:
    server: https://kubernetes.default.svc
    namespace: todo-workshop
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
EOF
```

Wait for the CNPG cluster to be healthy:

```bash
oc get cluster postgres -n todo-workshop -w
```

When `STATUS` shows `Cluster in healthy state`, you're good. This may take a minute or two.

### 2.2 Connect the App to PostgreSQL

Patch the ConfigMap and restart:

```bash
oc patch configmap todo-config -n todo-workshop \
  -p '{"data":{"DATABASE_URL":"postgres://todos:todos@postgres-rw:5432/todos?sslmode=disable"}}'
oc rollout restart deployment/todo-app -n todo-workshop
oc rollout status deployment/todo-app -n todo-workshop
```

### 2.3 Verify the Connection

Refresh the browser. The banner should now be **green**: **"Storage: PostgreSQL (persistent)"**.

Check the logs:

```bash
oc logs -l app=todo-app -n todo-workshop
```

```
Using PostgreSQL store
Listening on :8080
```

### 2.4 Test Persistence

Add a few todos. Then kill the pod again:

```bash
oc delete pod -l app=todo-app -n todo-workshop
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

Delete the ArgoCD applications (this removes all synced resources):

```bash
oc delete application todo-database -n openshift-gitops
oc delete application todo-app -n openshift-gitops
oc delete project todo-workshop
```
