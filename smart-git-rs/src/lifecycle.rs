use std::{
    collections::HashMap,
    path::Path,
    sync::{Arc, Mutex},
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use git_server_core::discovery::RepoInfo;
use git_server_http::{SharedState as GitHttpState, error::AppError};
use tokio::sync::Mutex as AsyncMutex;

use crate::{db::Database, git::MirrorService, model::RepoCacheRecord, repo_id::RepoId};

pub struct RepositoryLifecycleManager {
    db: Arc<Database>,
    mirror_service: Arc<MirrorService>,
    git_http_state: GitHttpState,
    refresh_ttl: Duration,
    repo_locks: Mutex<HashMap<String, Arc<AsyncMutex<()>>>>,
}

impl RepositoryLifecycleManager {
    pub fn new(
        db: Arc<Database>,
        mirror_service: Arc<MirrorService>,
        git_http_state: GitHttpState,
        refresh_ttl: Duration,
    ) -> Self {
        Self {
            db,
            mirror_service,
            git_http_state,
            refresh_ttl,
            repo_locks: Mutex::new(HashMap::new()),
        }
    }

    pub fn git_http_state(&self) -> &GitHttpState {
        &self.git_http_state
    }

    pub async fn sync_for_request(&self, repo_id: &RepoId) -> Result<SyncRegistration, AppError> {
        self.sync_with_policy(repo_id, SyncTrigger::Request, RefreshPolicy::Ttl)
            .await
    }

    pub async fn sync_manual(&self, repo_id: &RepoId) -> Result<SyncRegistration, AppError> {
        self.sync_with_policy(repo_id, SyncTrigger::Manual, RefreshPolicy::Force)
            .await
    }

    pub async fn refresh_stale_repos(&self) -> anyhow::Result<usize> {
        let now = unix_timestamp()?;
        let stale_before = now - self.refresh_ttl.as_secs() as i64;
        let stale_records = self.db.list_stale_cache_records(stale_before)?;
        let mut refreshed = 0usize;

        for record in stale_records {
            let repo_id = RepoId::new(record.owner, record.name)?;
            match self
                .sync_with_policy(&repo_id, SyncTrigger::Background, RefreshPolicy::Ttl)
                .await
            {
                Ok(outcome) if outcome.refreshed => refreshed += 1,
                Ok(_) => {}
                Err(error) => {
                    tracing::warn!(
                        owner = repo_id.owner(),
                        repo = repo_id.name(),
                        error = ?error,
                        "background refresh failed"
                    );
                }
            }
        }

        Ok(refreshed)
    }

    async fn sync_with_policy(
        &self,
        repo_id: &RepoId,
        trigger: SyncTrigger,
        refresh_policy: RefreshPolicy,
    ) -> Result<SyncRegistration, AppError> {
        let repo_lock = self.repo_lock(repo_id);
        let _guard = repo_lock.lock().await;

        let cached = self.db.get_cache_record(repo_id).map_err(to_internal)?;
        let should_refresh = self.should_refresh(cached.as_ref(), refresh_policy)?;
        let outcome = if should_refresh {
            Some(self.mirror_service.sync(repo_id).map_err(to_internal)?)
        } else {
            None
        };
        let fresh_clone = outcome.as_ref().is_some_and(|outcome| outcome.fresh_clone);

        let record = match (cached, outcome) {
            (Some(record), None) => record,
            (_, Some(outcome)) => RepoCacheRecord {
                owner: repo_id.owner().to_owned(),
                name: repo_id.name().to_owned(),
                upstream_url: outcome.upstream_url,
                local_path: outcome.local_path.display().to_string(),
                head_oid: outcome.head_oid,
                updated_at: unix_timestamp().map_err(to_internal)?,
            },
            (None, None) => {
                return Err(AppError::Internal(
                    "cache state inconsistent: missing repo without refresh".to_owned(),
                ));
            }
        };

        self.db
            .apply_lifecycle_update(
                repo_id,
                should_refresh.then_some(&record),
                trigger.count_request(),
                fresh_clone,
            )
            .map_err(to_internal)?;

        let repo_info = repo_info_from_record(&record);
        match self.git_http_state.register_repo(repo_info.clone()) {
            Ok(()) => {}
            Err(git_server_core::error::Error::Protocol(message))
                if message.starts_with("repository already registered:") => {}
            Err(error) => return Err(error.into()),
        }

        Ok(SyncRegistration {
            record,
            repo_info,
            fresh_clone,
            refreshed: should_refresh,
        })
    }

    fn should_refresh(
        &self,
        record: Option<&RepoCacheRecord>,
        refresh_policy: RefreshPolicy,
    ) -> Result<bool, AppError> {
        if matches!(refresh_policy, RefreshPolicy::Force) {
            return Ok(true);
        }

        let Some(record) = record else {
            return Ok(true);
        };

        if !local_repo_is_usable(&record.local_path) {
            return Ok(true);
        }

        let now = unix_timestamp().map_err(to_internal)?;
        Ok(now.saturating_sub(record.updated_at) >= self.refresh_ttl.as_secs() as i64)
    }

    fn repo_lock(&self, repo_id: &RepoId) -> Arc<AsyncMutex<()>> {
        let key = format!("{}/{}", repo_id.owner(), repo_id.name());
        let mut repo_locks = self
            .repo_locks
            .lock()
            .expect("repo lifecycle lock map poisoned");
        repo_locks
            .entry(key)
            .or_insert_with(|| Arc::new(AsyncMutex::new(())))
            .clone()
    }
}

pub struct SyncRegistration {
    pub record: RepoCacheRecord,
    pub repo_info: RepoInfo,
    pub fresh_clone: bool,
    pub refreshed: bool,
}

#[derive(Copy, Clone)]
enum SyncTrigger {
    Request,
    Manual,
    Background,
}

impl SyncTrigger {
    fn count_request(self) -> bool {
        matches!(self, Self::Request | Self::Manual)
    }
}

#[derive(Copy, Clone)]
enum RefreshPolicy {
    Ttl,
    Force,
}

fn repo_info_from_record(record: &RepoCacheRecord) -> RepoInfo {
    RepoInfo {
        name: format!("{}.git", record.name),
        relative_path: format!("{}/{}.git", record.owner, record.name),
        absolute_path: std::path::PathBuf::from(&record.local_path),
        description: None,
    }
}

fn local_repo_is_usable(path: &str) -> bool {
    let path = Path::new(path);
    if !path.exists() {
        return false;
    }

    match gix::open(path) {
        Ok(repo) => repo.is_bare(),
        Err(_) => false,
    }
}

fn unix_timestamp() -> anyhow::Result<i64> {
    let duration = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|error| anyhow::anyhow!(error))?;
    Ok(duration.as_secs() as i64)
}

fn to_internal(error: anyhow::Error) -> AppError {
    tracing::error!(error = %error, "git mirror operation failed");
    AppError::Internal("internal server error".to_owned())
}
