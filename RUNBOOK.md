# RUNBOOK.md - book-review-publisher

## Overview

Webhook service deployed to the k3s homelab cluster. Receives book review
requests and publishes them to `benniemosher/me` via GitHub API.

- **Namespace:** `book-review-publisher`
- **Internal URL:** `http://book-review-publisher.book-review-publisher.svc.cluster.local`
- **Cluster URL:** `http://book-review-publisher.mosher-labs.local/publish`
- **Image:** `ghcr.io/mosher-labs/book-review-publisher:latest`

## Initial deployment

### 1. Create the GitHub PAT

Create a fine-grained GitHub PAT at <https://github.com/settings/tokens> with:

- **Resource owner:** `benniemosher`
- **Repository access:** `benniemosher/me` only
- **Permissions:** Contents (read/write), Pull requests (read/write)

### 2. Create the Kubernetes sealed secret

```bash
# Create the plain secret (do NOT commit this)
kubectl create secret generic book-review-publisher-secrets \
  --namespace book-review-publisher \
  --from-literal=github-pat=ghp_YOUR_PAT_HERE \
  --dry-run=client -o yaml > /tmp/secret.yaml

# Seal it (requires kubeseal and cluster access)
kubeseal \
  --controller-namespace kube-system \
  --controller-name sealed-secrets-controller \
  --format yaml \
  < /tmp/secret.yaml \
  > manifests/sealed-secret.yaml

# Clean up the plain secret
rm /tmp/secret.yaml
```

Commit `manifests/sealed-secret.yaml` — it is safe to store in git.

### 3. Add ArgoCD Application to homelab-gitops

Copy `argocd/application.yaml` to
`apps/book-review-publisher/application.yaml` in the
`mosher-labs/homelab-gitops` repo and commit it. ArgoCD will automatically
sync and deploy the service.

### 4. Verify deployment

```bash
kubectl get pods -n book-review-publisher
kubectl logs -n book-review-publisher deploy/book-review-publisher
curl http://book-review-publisher.mosher-labs.local/health
```

## Updating the service

1. Merge a PR to `main` — the release workflow creates a new GitHub release
   and the Docker image is rebuilt and pushed to GHCR.
2. Update the image tag in `manifests/deployment.yaml` to pin to the new
   version, or leave as `latest` and ArgoCD will pull it on next sync.

## Rotating the GitHub PAT

1. Create a new PAT (see step 1 above).
2. Re-seal and re-commit `manifests/sealed-secret.yaml`.
3. ArgoCD syncs the new sealed secret; the pod restarts automatically.

## Troubleshooting

### Pod not starting

```bash
kubectl describe pod -n book-review-publisher -l app.kubernetes.io/name=book-review-publisher
```

Common cause: `book-review-publisher-secrets` secret not found — ensure the
sealed secret was committed and synced before the deployment.

### Publish request fails with 500

Check pod logs:

```bash
kubectl logs -n book-review-publisher deploy/book-review-publisher
```

Common causes:

- **`get ref: 404`** — the PAT does not have access to `benniemosher/me`.
- **`create branch: 422`** — branch `book-review/{date}-{slug}` already exists;
  delete it in the target repo and retry.
- **`commit image: 422`** — the image file already exists on the branch.

### Checking the PAT works

```bash
kubectl exec -n book-review-publisher deploy/book-review-publisher -- \
  sh -c 'curl -s -H "Authorization: token $GITHUB_PAT" https://api.github.com/repos/benniemosher/me | jq .full_name'
```

Expected output: `"benniemosher/me"`
