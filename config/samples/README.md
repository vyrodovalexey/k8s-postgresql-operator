# Config Samples - Apply Sequence

These samples are configured for the test environment (docker-compose).

## Prerequisites

1. Test environment running: `docker compose -f test/docker-compose/docker-compose.yml up -d`
2. Vault configured: `./test/scripts/setup-vault.sh && ./test/scripts/setup-vault-pki.sh`
3. Operator deployed to K8s with Vault PKI enabled

## Correct Apply Sequence

Resources must be applied in dependency order:

### Step 1: Register PostgreSQL Instance

```bash
kubectl apply -f config/samples/postgresql.yaml
```

Wait for the PostgreSQL instance to show "Connected" status.

### Step 2: Create User

```bash
kubectl apply -f config/samples/user.yaml
```

The operator will create the user in PostgreSQL and store credentials in Vault.

### Step 3: Create Database with Schema

```bash
kubectl apply -f config/samples/database.yaml
```

Creates database `testdb` owned by `testuser` with schema `testschema`.

### Step 4: Create Schema (standalone)

```bash
kubectl apply -f config/samples/schema.yaml
```

### Step 5: Create Grant (with prerequisite User)

```bash
kubectl apply -f config/samples/grant.yaml
```

This creates user `testreader` and grants SELECT privileges on `testdb`.

## Negative Tests (Expected to Fail)

These samples test webhook validation:

```bash
# Wrong PostgreSQL ID - webhook should reject
kubectl apply -f config/samples/user_wrong_postgresqlid.yaml

# Wrong database reference - webhook should reject
kubectl apply -f config/samples/grants_wrong_database.yaml

# Wrong PostgreSQL ID for grant - webhook should reject
kubectl apply -f config/samples/grant_wrong_postgresqlid.yaml
```

## Cleanup

```bash
kubectl delete -f config/samples/grant.yaml
kubectl delete -f config/samples/schema.yaml
kubectl delete -f config/samples/database.yaml
kubectl delete -f config/samples/user.yaml
kubectl delete -f config/samples/postgresql.yaml
```
