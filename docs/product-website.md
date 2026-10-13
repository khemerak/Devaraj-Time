# Product website and repository publication

The public Devaraj Time showcase lives in the separate Devaraj Solutions website at `/products/devaraj-time`. It explains the target audiences, core workflows, benefits, and current limitations, without requiring application access or sharing the attendance database.

Visitor flow: Devaraj Solutions product card → public product showcase → configured application URL or product inquiry. Application access remains administrator-provisioned; the showcase does not create a public registration flow.

The company website uses NEXT_PUBLIC_DEVARAJ_TIME_URL for the actual application entry point. Leave it empty until a verified URL is known. Set DEVARAJ_TIME_AVAILABLE=true only after production verification. The showcase can be public while this application repository remains private.

## Repository preparation
The application has a local Git repository. Runtime .env files, local state, generated binaries, dependency folders, certificates and key files are excluded. Include source code, tests, lockfiles, example configuration, Docker definitions and documentation in the initial commit. Inspect staged changes before committing. Use the owner's supplied author name/email; do not invent an identity.

A repository URL is required before adding origin and pushing. No remote URL is assumed. Do not overwrite an existing origin. Hosting requires the owner's chosen destination, domain and access. Keep production database and SMTP credentials in host configuration, never in Git.

## Launch verification
Follow deployment.md and limitations.md for application operating requirements. Verify HTTPS, migrations, backup/restore, least-privilege database access, company provisioning, real mobile camera/location behavior, account recovery, and applicable privacy requirements before declaring the application publicly available. Do not seed fictional development accounts into production.
