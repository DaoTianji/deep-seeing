#!/usr/bin/env python3
"""Provision only Deep-Seeing's isolated PostgreSQL. Run as root on the server.

Never prints credentials. Does not pull images, replace containers or touch any
other database. Reruns validate the exact existing container before using it.
"""
import json
import os
from pathlib import Path
import secrets
import subprocess
import time

NAME = "deep-seeing-memory-pg"
VOLUME = "deep-seeing-memory-pgdata"
IMAGE = "pgvector/pgvector:0.8.5-pg16"
LABEL = "deep-seeing-memory"
ROOT = Path("/etc/deep-seeing/memory")


def run(*args, input=None):
    result = subprocess.run(args, input=input, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError("operation failed: " + args[0] + " " + args[1])
    return result.stdout.strip()


def secret_file(path, content):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as f:
        f.write(content)


def main():
    if os.geteuid() != 0:
        raise RuntimeError("root required")
    image = json.loads(run("docker", "image", "inspect", IMAGE))[0]
    existing = run("docker", "ps", "-a", "--filter", "name=^/" + NAME + "$", "--format", "{{.Names}}")
    if existing:
        info = json.loads(run("docker", "inspect", NAME))[0]
        if info["Config"].get("Labels", {}).get("com.deep-seeing.purpose") != LABEL:
            raise RuntimeError("container name already owned by another service")
        binding = info["HostConfig"]["PortBindings"].get("5432/tcp", [])
        if binding != [{"HostIp": "127.0.0.1", "HostPort": "5457"}]:
            raise RuntimeError("unexpected existing port binding")
        if info["Image"] != image["Id"]:
            raise RuntimeError("existing image differs; manual review required")
        if not info["State"]["Running"]:
            raise RuntimeError("existing container is stopped; manual review required")
    ROOT.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(ROOT, 0o700)
    config = ROOT / "postgres.env"
    app_secret = ROOT / "hindsight-db-password"
    if not config.exists():
        if existing:
            raise RuntimeError("existing container has no owned configuration")
        secret_file(config, "POSTGRES_USER=ds_memory_admin\nPOSTGRES_DB=hindsight\nPOSTGRES_PASSWORD=" + secrets.token_hex(32) + "\n")
    if not app_secret.exists():
        if existing:
            raise RuntimeError("existing container has no owned app credential")
        secret_file(app_secret, secrets.token_hex(32))
    if not existing:
        volumes = run("docker", "volume", "ls", "--filter", "name=^" + VOLUME + "$", "--format", "{{.Name}}")
        if volumes:
            raise RuntimeError("volume already exists without container; refuse reuse")
        run("docker", "volume", "create", "--label", "com.deep-seeing.purpose=" + LABEL, VOLUME)
        run("docker", "run", "-d", "--name", NAME, "--label", "com.deep-seeing.purpose=" + LABEL,
            "--restart", "unless-stopped", "--memory", "768m", "--cpus", "1", "--shm-size", "128m",
            "--log-driver", "json-file", "--log-opt", "max-size=10m", "--log-opt", "max-file=3",
            "--env-file", str(config), "-p", "127.0.0.1:5457:5432", "-v", VOLUME + ":/var/lib/postgresql/data",
            "--health-cmd", "pg_isready -h 127.0.0.1 -U ds_memory_admin -d hindsight", "--health-interval", "10s",
            "--health-timeout", "3s", "--health-retries", "5", image["Id"],
            "postgres", "-c", "shared_buffers=128MB", "-c", "max_connections=40")
    for _ in range(30):
        # The image temporarily starts a Unix-socket-only server during init.
        # Wait for TCP so we cannot race its shutdown and the final server start.
        check = subprocess.run(["docker", "exec", NAME, "pg_isready", "-h", "127.0.0.1", "-U", "ds_memory_admin", "-d", "hindsight"], capture_output=True)
        if check.returncode == 0:
            break
        time.sleep(1)
    else:
        raise RuntimeError("database did not become ready")
    password = app_secret.read_text().strip()
    if len(password) != 64 or any(c not in "0123456789abcdef" for c in password):
        raise RuntimeError("invalid owned credential")
    sql = """DO $$ BEGIN
      IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'hindsight') THEN
        CREATE ROLE hindsight LOGIN PASSWORD '%s' NOSUPERUSER NOCREATEDB NOCREATEROLE;
      END IF;
    END $$;
    ALTER DATABASE hindsight OWNER TO hindsight;
    CREATE EXTENSION IF NOT EXISTS vector;
    GRANT ALL ON SCHEMA public TO hindsight;
    SELECT extversion FROM pg_extension WHERE extname='vector';
    SELECT '[1,0,0]'::vector <=> '[0,1,0]'::vector;
    """ % password
    # Credentials go through stdin, never argv or console output.
    result = run("docker", "exec", "-i", NAME, "psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "ds_memory_admin", "-d", "hindsight", input=sql)
    print(json.dumps({"container": NAME, "image_id": image["Id"], "listen": "127.0.0.1:5457", "volume": VOLUME, "vector_check": result.splitlines()[-3:]}, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(str(exc))
        raise SystemExit(1)
