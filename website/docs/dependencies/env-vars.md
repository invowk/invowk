---
sidebar_position: 6
---

# Environment Variable Dependencies

Environment variable dependencies verify that required environment variables exist before your command runs. They're checked **first**, before any other dependencies, against the user's actual environment.

## Basic Usage

```cue
{
    name: "deploy"
    depends_on: {
        env_vars: [
            {alternatives: [{name: "AWS_ACCESS_KEY_ID"}]}
        ]
    }
    implementations: [...]
}
```

If the variable isn't set:

```
✗ Dependencies not satisfied

Command 'deploy' has unmet dependencies:

Missing Environment Variables:
  • AWS_ACCESS_KEY_ID - not set in environment

Set the required environment variables and try again.
```

## Alternatives (OR Semantics)

Require one of multiple variables:

```cue
depends_on: {
    env_vars: [
        // Either AWS_ACCESS_KEY_ID OR AWS_PROFILE
        {alternatives: [
            {name: "AWS_ACCESS_KEY_ID"},
            {name: "AWS_PROFILE"}
        ]}
    ]
}
```

The dependency is satisfied if **any** alternative exists.

## Regex Validation

Validate the variable's value matches a pattern:

```cue
depends_on: {
    env_vars: [
        // Must be set AND match semver format
        {alternatives: [{
            name: "VERSION"
            validation: "^[0-9]+\\.[0-9]+\\.[0-9]+$"
        }]}
    ]
}
```

If the value doesn't match:

```
✗ Dependencies not satisfied

Command 'release' has unmet dependencies:

Invalid Environment Variables:
  • VERSION - value "invalid" does not match pattern "^[0-9]+\.[0-9]+\.[0-9]+$"
```

## Validation Order

Environment variables are checked **first**, before all other dependencies:

1. **env_vars** ← Checked first!
2. tools
3. filepaths
4. capabilities
5. commands
6. custom_checks

This ensures you're validating against the user's actual environment, not variables set by Invowk's `env` construct.

## Real-World Examples

### AWS Credentials

```cue
{
    name: "deploy"
    description: "Deploy to AWS"
    depends_on: {
        env_vars: [
            // Need either access key or profile
            {alternatives: [
                {name: "AWS_ACCESS_KEY_ID"},
                {name: "AWS_PROFILE"}
            ]},
            // Region is required
            {alternatives: [{name: "AWS_REGION"}]}
        ]
        tools: [{alternatives: ["aws"]}]
    }
    implementations: [{
        script: "aws s3 sync ./dist s3://my-bucket"
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Database Connection

```cue
{
    name: "db migrate"
    description: "Run database migrations"
    depends_on: {
        env_vars: [
            {alternatives: [{
                name: "DATABASE_URL"
                // Validate it looks like a connection string
                validation: "^postgres(ql)?://.*$"
            }]}
        ]
        tools: [{alternatives: ["migrate", "goose"]}]
    }
    implementations: [{
        script: "migrate -path ./migrations -database $DATABASE_URL up"
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### API Keys

```cue
{
    name: "publish"
    description: "Publish package to registry"
    depends_on: {
        env_vars: [
            // NPM token for publishing
            {alternatives: [{name: "NPM_TOKEN"}]},
        ]
        tools: [{alternatives: ["npm"]}]
    }
    implementations: [{
        script: """
            echo "//registry.npmjs.org/:_authToken=${NPM_TOKEN}" > ~/.npmrc
            npm publish
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Environment-Specific Config

```cue
{
    name: "deploy"
    description: "Deploy to target environment"
    depends_on: {
        env_vars: [
            // DEPLOY_ENV must be one of: dev, staging, prod
            {alternatives: [{
                name: "DEPLOY_ENV"
                validation: "^(dev|staging|prod)$"
            }]}
        ]
    }
    implementations: [{
        script: """
            echo "Deploying to $DEPLOY_ENV..."
            ./scripts/deploy-$DEPLOY_ENV.sh
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Version Validation

```cue
{
    name: "release"
    description: "Create a release"
    depends_on: {
        env_vars: [
            // Version must be semantic
            {alternatives: [{
                name: "VERSION"
                validation: "^v?[0-9]+\\.[0-9]+\\.[0-9]+(-[a-zA-Z0-9]+)?$"
            }]},
            // Git tag message
            {alternatives: [{name: "RELEASE_NOTES"}]}
        ]
    }
    implementations: [{
        script: """
            git tag -a "$VERSION" -m "$RELEASE_NOTES"
            git push origin "$VERSION"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Common Validation Patterns

### Semantic Version

```cue
validation: "^[0-9]+\\.[0-9]+\\.[0-9]+$"
// Matches: 1.0.0, 2.1.3
// Rejects: v1.0.0, 1.0, latest
```

### URL

```cue
validation: "^https?://[^\\s]+$"
// Matches: http://localhost, https://example.com/path
// Rejects: ftp://server, not-a-url
```

### Email-like

```cue
validation: "^[^@]+@[^@]+\\.[^@]+$"
// Matches: user@example.com
// Rejects: invalid, @example.com
```

### Alphanumeric ID

```cue
validation: "^[a-zA-Z0-9_-]+$"
// Matches: my-project_123, ABC
// Rejects: my project, name@here
```

### AWS Region

```cue
validation: "^[a-z]{2}-[a-z]+-[0-9]+$"
// Matches: us-east-1, eu-west-2
// Rejects: US-EAST-1, us_east_1
```

## Multiple Requirements

Combine multiple env var checks (AND logic):

```cue
depends_on: {
    env_vars: [
        // Need API_KEY AND API_SECRET AND API_URL
        {alternatives: [{name: "API_KEY"}]},
        {alternatives: [{name: "API_SECRET"}]},
        {alternatives: [{
            name: "API_URL"
            validation: "^https://.*$"
        }]},
    ]
}
```

## Important: User Environment Only

Environment variable dependencies check the **user's environment**, not variables set by Invowk:

```cue
{
    name: "example"
    env: {
        vars: {
            // This is set by Invowk during execution
            MY_VAR: "value"
        }
    }
    depends_on: {
        env_vars: [
            // This checks the USER's environment, BEFORE Invowk sets MY_VAR
            // So it will fail if the user hasn't set MY_VAR themselves
            {alternatives: [{name: "MY_VAR"}]}
        ]
    }
}
```

This is intentional - you're validating what the user has configured, not what your command will set.

## Best Practices

1. **Use alternatives for auth methods**: `{alternatives: [{name: "TOKEN"}, {name: "API_KEY"}]}`
2. **Add validation when format matters**: Especially for URLs, versions, and IDs
3. **Document required vars**: Users need to know what to set
4. **Consider secrets management**: Don't log sensitive values

## Next Steps

- [Custom Checks](./custom-checks) - Write custom validation scripts
- [Overview](./overview) - Return to dependencies overview
