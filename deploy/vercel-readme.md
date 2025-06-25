# Vercel Deployment Notes

Vercel doesn't support native cron jobs for free tier.
You'll need to use external services to trigger the function:

## External Cron Services (Free)

1. **GitHub Actions** (Recommended)
2. **UptimeRobot**
3. **Cron-job.org**
4. **EasyCron**

## Example GitHub Actions Workflow

Create `.github/workflows/trigger-vercel.yml`:

```yaml
name: Trigger TA Watcher
on:
  schedule:
    - cron: "*/5 * * * *" # Every 5 minutes
  workflow_dispatch: # Manual trigger

jobs:
  trigger:
    runs-on: ubuntu-latest
    steps:
      - name: Trigger Vercel Function
        run: |
          curl -X POST https://your-vercel-app.vercel.app/api/trigger
```

## Environment Variables

Set these in your Vercel dashboard:

- `SMTP_HOST`
- `SMTP_USERNAME`
- `SMTP_PASSWORD`
- `FROM_EMAIL`
- `TO_EMAIL`

## Deployment Command

```bash
vercel --prod
```
