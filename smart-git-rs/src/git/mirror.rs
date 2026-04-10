use std::{
    fs,
    path::{Path, PathBuf},
    sync::atomic::AtomicBool,
};

use anyhow::Context;
use gix::progress::Discard;

use crate::repo_id::RepoId;

#[derive(Debug, Clone)]
pub struct SyncOutcome {
    pub upstream_url: String,
    pub local_path: PathBuf,
    pub head_oid: Option<String>,
    pub fresh_clone: bool,
}

#[derive(Debug)]
pub struct MirrorService {
    repo_root: PathBuf,
    github_base: String,
}

impl MirrorService {
    pub fn new(repo_root: PathBuf, github_base: String) -> Self {
        Self {
            repo_root,
            github_base: github_base.trim_end_matches('/').to_owned(),
        }
    }

    pub fn sync(&self, repo_id: &RepoId) -> anyhow::Result<SyncOutcome> {
        let upstream_url = self.upstream_url(repo_id);
        let local_path = self.repo_root.join(repo_id.local_rel_path());

        if local_path.exists() {
            match gix::open(&local_path) {
                Ok(repo) if repo.is_bare() => {
                    return self.fetch_existing(repo, &upstream_url, &local_path);
                }
                Err(_) => remove_invalid_path(&local_path)?,
                Ok(_) => remove_invalid_path(&local_path)?,
            }
        }

        if let Some(parent) = local_path.parent() {
            fs::create_dir_all(parent).with_context(|| {
                format!("failed to create repository parent {}", parent.display())
            })?;
        }

        let should_interrupt = AtomicBool::new(false);
        let mut prepare = gix::prepare_clone_bare(upstream_url.as_str(), &local_path)
            .with_context(|| format!("failed to clone {}", upstream_url))?;
        let (repo, _) = prepare
            .fetch_only(Discard, &should_interrupt)
            .with_context(|| format!("failed to fetch initial mirror from {}", upstream_url))?;

        Ok(SyncOutcome {
            upstream_url,
            local_path,
            head_oid: repo_head_oid(&repo),
            fresh_clone: true,
        })
    }

    pub fn local_path(&self, repo_id: &RepoId) -> PathBuf {
        self.repo_root.join(repo_id.local_rel_path())
    }

    fn fetch_existing(
        &self,
        repo: gix::Repository,
        upstream_url: &str,
        local_path: &Path,
    ) -> anyhow::Result<SyncOutcome> {
        let should_interrupt = AtomicBool::new(false);
        let remote = repo
            .remote_at(upstream_url)
            .with_context(|| format!("failed to prepare remote {}", upstream_url))?
            .with_refspecs(Some("+refs/*:refs/*"), gix::remote::Direction::Fetch)
            .context("failed to configure mirror fetch refspec")?;
        let connection = remote
            .connect(gix::remote::Direction::Fetch)
            .context("failed to connect to upstream remote")?;
        let prepare = connection
            .prepare_fetch(Discard, Default::default())
            .context("failed to prepare mirror fetch")?;
        prepare
            .receive(Discard, &should_interrupt)
            .context("failed to receive mirror fetch pack")?;

        Ok(SyncOutcome {
            upstream_url: upstream_url.to_owned(),
            local_path: local_path.to_path_buf(),
            head_oid: repo_head_oid(&repo),
            fresh_clone: false,
        })
    }

    pub fn upstream_url(&self, repo_id: &RepoId) -> String {
        format!(
            "{}/{}/{}.git",
            self.github_base,
            repo_id.owner(),
            repo_id.name()
        )
    }
}
fn repo_head_oid(repo: &gix::Repository) -> Option<String> {
    repo.head()
        .ok()
        .and_then(|head| head.id())
        .map(|oid| oid.to_string())
}

fn remove_invalid_path(path: &Path) -> anyhow::Result<()> {
    let metadata = fs::symlink_metadata(path).with_context(|| {
        format!(
            "failed to inspect invalid repository path {}",
            path.display()
        )
    })?;

    if metadata.is_dir() {
        fs::remove_dir_all(path).with_context(|| {
            format!("failed to remove invalid repository dir {}", path.display())
        })?;
    } else {
        fs::remove_file(path).with_context(|| {
            format!(
                "failed to remove invalid repository file {}",
                path.display()
            )
        })?;
    }

    Ok(())
}
