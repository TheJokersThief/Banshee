# What

- Bumps `version` in `package.json` to `2.0.0`
- Adds `homepage` field pointing to the GitHub repository
- Removes the deprecated `scripts.prepublish` field (replaced by `scripts.prepare` in npm 5+)

# Why

- Version 1.x reaches end-of-life and no longer receives security patches
- The `prepublish` hook ran unexpectedly during `npm install`; removing it in favour of `prepare` fixes this behaviour for all consumers
