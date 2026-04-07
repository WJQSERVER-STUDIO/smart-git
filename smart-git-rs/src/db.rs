use std::{
    path::Path,
    sync::{Mutex, MutexGuard},
};

use anyhow::Context;
use rusqlite::{params, Connection, Error as SqlError, ErrorCode};

use crate::{
    model::{RepoCacheRecord, RepoStatsRecord},
    repo_id::RepoId,
};

pub struct Database {
    conn: Mutex<Connection>,
}

impl Database {
    pub fn open(path: &Path) -> anyhow::Result<Self> {
        let conn = Connection::open(path)
            .with_context(|| format!("failed to open sqlite database {}", path.display()))?;
        Ok(Self {
            conn: Mutex::new(conn),
        })
    }

    pub fn init_schema(&self) -> anyhow::Result<()> {
        let conn = self.conn()?;
        conn.execute_batch(
            r#"
            PRAGMA journal_mode = WAL;

            CREATE TABLE IF NOT EXISTS repo_cache (
                owner TEXT NOT NULL,
                name TEXT NOT NULL,
                upstream_url TEXT NOT NULL,
                local_path TEXT NOT NULL,
                head_oid TEXT,
                created_at INTEGER NOT NULL DEFAULT 0,
                updated_at INTEGER NOT NULL,
                expires_at INTEGER NOT NULL DEFAULT 0,
                status TEXT NOT NULL DEFAULT 'synced',
                PRIMARY KEY (owner, name)
            );

            CREATE TABLE IF NOT EXISTS repo_stats (
                owner TEXT NOT NULL,
                name TEXT NOT NULL,
                clone_count INTEGER NOT NULL DEFAULT 0,
                request_count INTEGER NOT NULL DEFAULT 0,
                PRIMARY KEY (owner, name)
            );
            "#,
        )
        .context("failed to initialize sqlite schema")?;

        ignore_duplicate_column(
            conn.execute(
                "ALTER TABLE repo_cache ADD COLUMN status TEXT NOT NULL DEFAULT 'synced'",
                [],
            ),
            "status",
        )?;
        ignore_duplicate_column(
            conn.execute(
                "ALTER TABLE repo_cache ADD COLUMN created_at INTEGER NOT NULL DEFAULT 0",
                [],
            ),
            "created_at",
        )?;
        ignore_duplicate_column(
            conn.execute(
                "ALTER TABLE repo_cache ADD COLUMN expires_at INTEGER NOT NULL DEFAULT 0",
                [],
            ),
            "expires_at",
        )?;

        Ok(())
    }

    pub fn apply_lifecycle_update(
        &self,
        repo_id: &RepoId,
        cache_record: Option<&RepoCacheRecord>,
        count_request: bool,
        increment_clone: bool,
    ) -> anyhow::Result<()> {
        let mut conn = self.conn()?;
        let tx = conn
            .transaction()
            .context("failed to begin lifecycle transaction")?;

        if let Some(record) = cache_record {
            tx.execute(
                r#"
                INSERT INTO repo_cache (owner, name, upstream_url, local_path, head_oid, created_at, updated_at, expires_at, status)
                VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)
                ON CONFLICT(owner, name) DO UPDATE SET
                    upstream_url = excluded.upstream_url,
                    local_path = excluded.local_path,
                    head_oid = excluded.head_oid,
                    created_at = excluded.created_at,
                    updated_at = excluded.updated_at,
                    expires_at = excluded.expires_at,
                    status = excluded.status
                "#,
                params![
                    &record.owner,
                    &record.name,
                    &record.upstream_url,
                    &record.local_path,
                    &record.head_oid,
                    record.created_at,
                    record.updated_at,
                    record.expires_at,
                    &record.status,
                ],
            )
            .context("failed to write repo cache record in lifecycle transaction")?;
        }

        if count_request || increment_clone {
            tx.execute(
                r#"
                INSERT INTO repo_stats (owner, name, clone_count, request_count)
                VALUES (?1, ?2, 0, 0)
                ON CONFLICT(owner, name) DO NOTHING
                "#,
                params![repo_id.owner(), repo_id.name()],
            )
            .context("failed to initialize repo stats row in lifecycle transaction")?;

            if count_request {
                tx.execute(
                    "UPDATE repo_stats SET request_count = request_count + 1 WHERE owner = ?1 AND name = ?2",
                    params![repo_id.owner(), repo_id.name()],
                )
                .context("failed to increment request count in lifecycle transaction")?;
            }

            if increment_clone {
                tx.execute(
                    "UPDATE repo_stats SET clone_count = clone_count + 1 WHERE owner = ?1 AND name = ?2",
                    params![repo_id.owner(), repo_id.name()],
                )
                .context("failed to increment clone count in lifecycle transaction")?;
            }
        }

        tx.commit()
            .context("failed to commit lifecycle transaction")?;
        Ok(())
    }

    pub fn list_cache_records(&self) -> anyhow::Result<Vec<RepoCacheRecord>> {
        let conn = self.conn()?;
        let mut statement = conn
            .prepare(
                r#"
                SELECT owner, name, upstream_url, local_path, head_oid, created_at, updated_at, expires_at, status
                FROM repo_cache
                ORDER BY owner, name
                "#,
            )
            .context("failed to prepare cache query")?;

        let rows = statement
            .query_map([], |row| {
                Ok(RepoCacheRecord {
                    owner: row.get(0)?,
                    name: row.get(1)?,
                    upstream_url: row.get(2)?,
                    local_path: row.get(3)?,
                    head_oid: row.get(4)?,
                    created_at: row.get(5)?,
                    updated_at: row.get(6)?,
                    expires_at: row.get(7)?,
                    status: row.get(8)?,
                })
            })
            .context("failed to query cache records")?;

        let mut records = Vec::new();
        for row in rows {
            records.push(row.context("failed to decode cache record")?);
        }

        Ok(records)
    }

    pub fn get_cache_record(&self, repo_id: &RepoId) -> anyhow::Result<Option<RepoCacheRecord>> {
        let conn = self.conn()?;
        let mut statement = conn
            .prepare(
                r#"
                SELECT owner, name, upstream_url, local_path, head_oid, created_at, updated_at, expires_at, status
                FROM repo_cache
                WHERE owner = ?1 AND name = ?2
                "#,
            )
            .context("failed to prepare single cache lookup")?;

        let mut rows = statement
            .query(params![repo_id.owner(), repo_id.name()])
            .context("failed to query single cache record")?;

        let Some(row) = rows
            .next()
            .context("failed to iterate single cache record")?
        else {
            return Ok(None);
        };

        Ok(Some(RepoCacheRecord {
            owner: row.get(0)?,
            name: row.get(1)?,
            upstream_url: row.get(2)?,
            local_path: row.get(3)?,
            head_oid: row.get(4)?,
            created_at: row.get(5)?,
            updated_at: row.get(6)?,
            expires_at: row.get(7)?,
            status: row.get(8)?,
        }))
    }

    pub fn list_stale_cache_records(
        &self,
        stale_before: i64,
    ) -> anyhow::Result<Vec<RepoCacheRecord>> {
        let conn = self.conn()?;
        let mut statement = conn
            .prepare(
                r#"
                SELECT owner, name, upstream_url, local_path, head_oid, created_at, updated_at, expires_at, status
                FROM repo_cache
                WHERE updated_at <= ?1
                ORDER BY updated_at ASC, owner ASC, name ASC
                "#,
            )
            .context("failed to prepare stale cache query")?;

        let rows = statement
            .query_map(params![stale_before], |row| {
                Ok(RepoCacheRecord {
                    owner: row.get(0)?,
                    name: row.get(1)?,
                    upstream_url: row.get(2)?,
                    local_path: row.get(3)?,
                    head_oid: row.get(4)?,
                    created_at: row.get(5)?,
                    updated_at: row.get(6)?,
                    expires_at: row.get(7)?,
                    status: row.get(8)?,
                })
            })
            .context("failed to query stale cache records")?;

        let mut records = Vec::new();
        for row in rows {
            records.push(row.context("failed to decode stale cache record")?);
        }

        Ok(records)
    }

    pub fn list_pending_records(&self) -> anyhow::Result<Vec<RepoCacheRecord>> {
        let conn = self.conn()?;
        let mut statement = conn
            .prepare(
                r#"
                SELECT owner, name, upstream_url, local_path, head_oid, created_at, updated_at, expires_at, status
                FROM repo_cache
                WHERE status = 'pending'
                ORDER BY updated_at ASC, owner ASC, name ASC
                "#,
            )
            .context("failed to prepare pending cache query")?;

        let rows = statement
            .query_map([], |row| {
                Ok(RepoCacheRecord {
                    owner: row.get(0)?,
                    name: row.get(1)?,
                    upstream_url: row.get(2)?,
                    local_path: row.get(3)?,
                    head_oid: row.get(4)?,
                    created_at: row.get(5)?,
                    updated_at: row.get(6)?,
                    expires_at: row.get(7)?,
                    status: row.get(8)?,
                })
            })
            .context("failed to query pending cache records")?;

        let mut records = Vec::new();
        for row in rows {
            records.push(row.context("failed to decode pending cache record")?);
        }

        Ok(records)
    }

    pub fn list_synced_records(&self) -> anyhow::Result<Vec<RepoCacheRecord>> {
        let conn = self.conn()?;
        let mut statement = conn
            .prepare(
                r#"
                SELECT owner, name, upstream_url, local_path, head_oid, created_at, updated_at, expires_at, status
                FROM repo_cache
                WHERE status = 'synced'
                ORDER BY owner, name
                "#,
            )
            .context("failed to prepare synced cache query")?;

        let rows = statement
            .query_map([], |row| {
                Ok(RepoCacheRecord {
                    owner: row.get(0)?,
                    name: row.get(1)?,
                    upstream_url: row.get(2)?,
                    local_path: row.get(3)?,
                    head_oid: row.get(4)?,
                    created_at: row.get(5)?,
                    updated_at: row.get(6)?,
                    expires_at: row.get(7)?,
                    status: row.get(8)?,
                })
            })
            .context("failed to query synced cache records")?;

        let mut records = Vec::new();
        for row in rows {
            records.push(row.context("failed to decode synced cache record")?);
        }

        Ok(records)
    }

    pub fn delete_cache_record(&self, repo_id: &RepoId) -> anyhow::Result<()> {
        let conn = self.conn()?;
        conn.execute(
            "DELETE FROM repo_cache WHERE owner = ?1 AND name = ?2",
            params![repo_id.owner(), repo_id.name()],
        )
        .context("failed to delete repo cache record")?;
        Ok(())
    }

    pub fn list_stats(&self) -> anyhow::Result<Vec<RepoStatsRecord>> {
        let conn = self.conn()?;
        let mut statement = conn
            .prepare(
                r#"
                SELECT owner, name, clone_count, request_count
                FROM repo_stats
                ORDER BY owner, name
                "#,
            )
            .context("failed to prepare stats query")?;

        let rows = statement
            .query_map([], |row| {
                Ok(RepoStatsRecord {
                    owner: row.get(0)?,
                    name: row.get(1)?,
                    clone_count: row.get(2)?,
                    request_count: row.get(3)?,
                })
            })
            .context("failed to query repo stats")?;

        let mut records = Vec::new();
        for row in rows {
            records.push(row.context("failed to decode repo stats row")?);
        }

        Ok(records)
    }

    fn conn(&self) -> anyhow::Result<MutexGuard<'_, Connection>> {
        self.conn
            .lock()
            .map_err(|_| anyhow::anyhow!("sqlite mutex poisoned"))
    }
}

fn ignore_duplicate_column(result: rusqlite::Result<usize>, column: &str) -> anyhow::Result<()> {
    match result {
        Ok(_) => Ok(()),
        Err(SqlError::SqliteFailure(error, Some(message)))
            if error.code == ErrorCode::Unknown
                && message.contains(&format!("duplicate column name: {column}")) =>
        {
            Ok(())
        }
        Err(error) => Err(error).with_context(|| format!("failed to add column {column}")),
    }
}
