# DEPLOY — Cloud Run in Tokyo

⚠ **Why it is deployed this way is [`adr/0008`](adr/0008-public-exposure-is-cloud-run-in-tokyo.md).**
⚠ **This file is the commands, in the order they were run.** ⚠ **Change one, and say so in the PR.**

⚠ **Every command here acts on a real project, costs or exposes something, or both.**
⚠ **Run them deliberately; there is no dry-run mode** ([`verification.md`](../.claude/rules/verification.md)
§ An exercise must not change the world).

```sh
PROJECT=connect-doctor-511201
REGION=asia-northeast1
REPO=connect-doctor
SA=connect-doctor-runtime
IMAGE=$REGION-docker.pkg.dev/$PROJECT/$REPO/connect-doctor:$(git rev-parse --short HEAD)
BILLING=01C888-A9D074-5B7101
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

## 2. Every release

```sh
docker build -t $IMAGE .
docker push $IMAGE

# ⚠ --max-instances 1 bounds what the service can cost and what it can be made to do.
# ⚠ --concurrency 16 matches limits.ConcurrentChecks; a 17th request gets 503 server.busy.
# ⚠ --allow-unauthenticated makes it public. That is the product.
gcloud run deploy connect-doctor --image $IMAGE --region $REGION --project $PROJECT \
  --service-account $SA@$PROJECT.iam.gserviceaccount.com \
  --max-instances 1 --concurrency 16 --cpu 1 --memory 512Mi --timeout 60 \
  --allow-unauthenticated
```

## 3. After every release

```sh
URL=$(gcloud run services describe connect-doctor --region $REGION --project $PROJECT --format='value(status.url)')
curl -s "$URL/api/check?url=https://example.com" | head -20
curl -s "$URL/api/check?url=http://169.254.169.254/"        # must be refused
curl -s "$URL/api/check?url=http://metadata.google.internal/" # must be refused

# ⚠ The runtime account must still have no role bindings: this must print nothing.
gcloud projects get-iam-policy $PROJECT --flatten=bindings --filter="bindings.members:serviceAccount:$SA@" \
  --format='value(bindings.role)'
```

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
