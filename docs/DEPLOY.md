# DEPLOY — Cloud Run in Tokyo

⚠ **Why it is deployed this way is [`adr/0008`](adr/0008-public-exposure-is-cloud-run-in-tokyo.md).**
⚠ **This file is the commands, in the order they were run.** ⚠ **Change one, and say so in the PR.**

⚠ **Every command here acts on a real project, costs or exposes something, or both.**
⚠ **Run them deliberately; there is no dry-run mode** ([`verification.md`](../.claude/rules/verification.md)
§ An exercise must not change the world).

```sh
eval "$(scripts/release.sh env)"     # PROJECT REGION REPO SERVICE SA IMAGE_REPO — one copy, in the script
IMAGE=$IMAGE_REPO:$(git rev-parse --short HEAD)
BILLING=01C888-A9D074-5B7101
DEPLOYER=connect-doctor-deployer
```

## 1. Once per project

```sh
gcloud services enable run.googleapis.com artifactregistry.googleapis.com billingbudgets.googleapis.com \
  --project $PROJECT

gcloud artifacts repositories create $REPO --repository-format=docker --location=$REGION --project $PROJECT

# ⚠ A service account with NO roles. The metadata server hands its token to the container;
#   an account that can do nothing makes a leaked token worth nothing (adr/0008).
gcloud iam service-accounts create $SA --display-name="ConnectDoctor runtime (no roles)" --project $PROJECT

# ⚠ ¥500/month on this project only; an alert, not a cap (owner decision, hidetzu/connect-doctor#18).
gcloud billing budgets create --billing-account=$BILLING --display-name="connect-doctor monthly" \
  --budget-amount=500JPY --filter-projects=projects/$PROJECT \
  --threshold-rule=percent=0.5 --threshold-rule=percent=0.9 --threshold-rule=percent=1.0

gcloud auth configure-docker $REGION-docker.pkg.dev
```

### Release from GitHub Actions (once; [`adr/0011`](adr/0011-a-merge-to-main-releases-itself-through-a-candidate-revision-and-only-dependabot-merges-itself.md))

```sh
gcloud services enable iamcredentials.googleapis.com sts.googleapis.com --project $PROJECT

# ⚠ Public access is granted once here, so a release never needs to set IAM on the service.
gcloud run services add-iam-policy-binding $SERVICE --region $REGION --project $PROJECT \
  --member=allUsers --role=roles/run.invoker

# ⚠ The deployer is a separate account. ⚠ The runtime account keeps no roles (adr/0008).
gcloud iam service-accounts create $DEPLOYER --display-name="ConnectDoctor release (GitHub Actions)" --project $PROJECT
D=serviceAccount:$DEPLOYER@$PROJECT.iam.gserviceaccount.com
gcloud artifacts repositories add-iam-policy-binding $REPO --location $REGION --project $PROJECT \
  --member=$D --role=roles/artifactregistry.writer
# ⚠ Project-level: deploy also polls operations and lists revisions; this project has one service.
gcloud projects add-iam-policy-binding $PROJECT --member=$D --role=roles/run.developer --condition=None
# ⚠ actAs on the runtime account only, so a deploy can run as it and as nothing else.
gcloud iam service-accounts add-iam-policy-binding $SA@$PROJECT.iam.gserviceaccount.com --project $PROJECT \
  --member=$D --role=roles/iam.serviceAccountUser
# ⚠ Read-only: § 3 reads the project IAM policy to prove the runtime account has no roles.
gcloud projects add-iam-policy-binding $PROJECT --member=$D --role=roles/iam.securityReviewer --condition=None

# ⚠ No key file. GitHub's OIDC token is trusted only for this repository (by its immutable id),
#   on refs/heads/main, from release.yml.
gcloud iam workload-identity-pools create github --location=global --display-name="GitHub Actions" --project $PROJECT
gcloud iam workload-identity-pools providers create-oidc connect-doctor --location=global \
  --workload-identity-pool=github --project $PROJECT \
  --issuer-uri=https://token.actions.githubusercontent.com \
  --attribute-mapping=google.subject=assertion.sub,attribute.repository_id=assertion.repository_id,attribute.ref=assertion.ref,attribute.workflow_ref=assertion.workflow_ref \
  --attribute-condition="assertion.repository_id=='1412545843' && assertion.ref=='refs/heads/main' && assertion.workflow_ref.startsWith('hidetzu/connect-doctor/.github/workflows/release.yml@')"
NUM=$(gcloud projects describe $PROJECT --format='value(projectNumber)')
gcloud iam service-accounts add-iam-policy-binding $DEPLOYER@$PROJECT.iam.gserviceaccount.com --project $PROJECT \
  --role=roles/iam.workloadIdentityUser \
  --member="principalSet://iam.googleapis.com/projects/$NUM/locations/global/workloadIdentityPools/github/attribute.repository_id/1412545843"

# ⚠ Not secrets: they name the provider and the account, and grant nothing without the condition above.
gh variable set GCP_WIF_PROVIDER --body "projects/$NUM/locations/global/workloadIdentityPools/github/providers/connect-doctor"
gh variable set GCP_DEPLOYER_SA --body "$DEPLOYER@$PROJECT.iam.gserviceaccount.com"
```

## 2. Every release

⚠ **A push to `main` runs [`release.yml`](../.github/workflows/release.yml), which runs exactly
the steps below** ([`adr/0011`](adr/0011-a-merge-to-main-releases-itself-through-a-candidate-revision-and-only-dependabot-merges-itself.md)).
⚠ **By hand, it is the same script.** ⚠ **The deploy flags and their reasons live in
[`scripts/release.sh`](../scripts/release.sh) only.**

```sh
docker build -t $IMAGE .
docker push $IMAGE

# ⚠ A new revision with no traffic, tagged candidate; prints REVISION= and URL=.
eval "$(scripts/release.sh candidate $IMAGE $(git rev-parse --short HEAD)-manual-$(date +%s))"
```

## 3. Before traffic moves

```sh
# ⚠ § 3 checks, against the candidate's own URL: the forged X-Forwarded-For case first (it
#   needs this client's burst full), example.com ok, 169.254.169.254 and metadata.google.internal
#   refused, the runtime account with no roles. ⚠ Non-zero on any failure.
scripts/release.sh check "$URL"

# ⚠ Only after check passed. ⚠ By revision name: never "latest".
scripts/release.sh promote "$REVISION"

# ⚠ Always, pass or fail: a tagged revision is a second public endpoint with its own limits.
scripts/release.sh untag
```

⚠ **To check the serving revision instead**: `scripts/release.sh check "$(gcloud run services describe
$SERVICE --region $REGION --project $PROJECT --format='value(status.url)')"`.

## 4. Custom domain (once; [`adr/0009`](adr/0009-the-public-address-is-connect-doctor-hidetzu-work-through-cloud-run-domain-mapping.md))

```sh
gcloud components install beta
gcloud domains list-user-verified           # hidetzu.work must be listed
gcloud beta run domain-mappings create --service connect-doctor \
  --domain connect-doctor.hidetzu.work --region $REGION --project $PROJECT
# ⚠ Then, in Cloudflare: CNAME connect-doctor -> ghs.googlehosted.com, proxy OFF (DNS only).
# ⚠ The certificate is issued after the record resolves: about 15 minutes, up to 24 hours.
gcloud beta run domain-mappings describe --domain connect-doctor.hidetzu.work \
  --region $REGION --project $PROJECT --format='yaml(status.conditions)'
```
