use std::sync::Arc;

use anyhow::Context;
use git_server_core::discovery::DynamicRepoRegistry;
use git_server_http::{AuthConfig, ServicePolicy, SharedState as GitHttpState};
use tokio::net::TcpListener;
use tokio::time::MissedTickBehavior;
use tracing::info;

use crate::{
    config::Config, db::Database, git::MirrorService, http::build_router,
    lifecycle::RepositoryLifecycleManager,
};

#[derive(Clone)]
pub struct AppState {
    pub config: Config,
    pub db: Arc<Database>,
    pub lifecycle: Arc<RepositoryLifecycleManager>,
}

pub async fn run(config: Config) -> anyhow::Result<()> {
    config.ensure_dirs()?;

    let db = Arc::new(Database::open(&config.database.path)?);
    db.init_schema()?;

    let mirror_service = Arc::new(MirrorService::new(
        config.cache.repo_dir.clone(),
        config.upstream.github_base.clone(),
    ));
    let git_http_state = GitHttpState::with_registry(
        Arc::new(DynamicRepoRegistry::new()),
        AuthConfig::default(),
        ServicePolicy {
            upload_pack: true,
            upload_pack_v2: true,
            receive_pack: false,
        },
    );
    let lifecycle = Arc::new(RepositoryLifecycleManager::new(
        db.clone(),
        mirror_service,
        git_http_state,
        config.cache.refresh_ttl(),
    ));
    lifecycle.recover_pending().await?;

    let state = AppState {
        config,
        db,
        lifecycle,
    };

    spawn_refresh_loop(state.clone());

    let app = build_router(state.clone());
    let addr = state.config.server.listen_addr();
    let listener = TcpListener::bind(&addr)
        .await
        .with_context(|| format!("failed to bind {addr}"))?;

    info!(addr = %addr, "listening");

    axum::serve(listener, app)
        .with_graceful_shutdown(shutdown_signal())
        .await
        .context("http server exited unexpectedly")
}

fn spawn_refresh_loop(state: AppState) {
    let interval = state.config.cache.refresh_scan_interval();
    tokio::spawn(async move {
        let mut ticker = tokio::time::interval(interval);
        ticker.set_missed_tick_behavior(MissedTickBehavior::Skip);
        ticker.tick().await;

        loop {
            ticker.tick().await;
            match state.lifecycle.refresh_stale_repos().await {
                Ok(refreshed) if refreshed > 0 => {
                    tracing::info!(refreshed, "background refresh completed")
                }
                Ok(_) => {}
                Err(error) => tracing::warn!(error = %error, "background refresh scan failed"),
            }
        }
    });
}

async fn shutdown_signal() {
    let ctrl_c = async {
        let _ = tokio::signal::ctrl_c().await;
    };

    #[cfg(unix)]
    let terminate = async {
        use tokio::signal::unix::{SignalKind, signal};

        if let Ok(mut signal) = signal(SignalKind::terminate()) {
            signal.recv().await;
        }
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c => {}
        _ = terminate => {}
    }
}
