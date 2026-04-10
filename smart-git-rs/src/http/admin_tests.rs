#[cfg(test)]
mod tests {
    use std::{path::PathBuf, sync::Arc};

    use axum::{Router, body::to_bytes, http::{Request, StatusCode}, routing::get};
    use gitserver_core::discovery::DynamicRepoRegistry;
    use gitserver_http::{AuthConfig, ServicePolicy, SharedState as GitHttpState};
    use tower::ServiceExt;

    use crate::{
        app::AppState,
        config::{CacheConfig, Config, DatabaseConfig, ServerConfig, UpstreamConfig},
        db::Database,
        git::MirrorService,
        http::admin,
        lifecycle::RepositoryLifecycleManager,
    };

    #[tokio::test]
    async fn list_cache_records_internal_error_should_not_map_to_bad_request() {
        let (state, _dir) = broken_admin_state();
        let app = Router::new()
            .route("/api/db/data", get(admin::list_cache_records))
            .with_state(state);

        let response = app
            .oneshot(
                Request::builder()
                    .uri("/api/db/data")
                    .body(axum::body::Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();

        assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);

        let content_type = response
            .headers()
            .get(axum::http::header::CONTENT_TYPE)
            .and_then(|value| value.to_str().ok())
            .unwrap_or_default();
        assert!(content_type.contains("application/vnd.wjqserver.wanf"));

        let body = to_bytes(response.into_body(), usize::MAX).await.unwrap();
        let text = String::from_utf8(body.to_vec()).unwrap();
        assert!(text.contains("error = "));
    }

    fn broken_admin_state() -> (AppState, tempfile::TempDir) {
        let dir = tempfile::tempdir().unwrap();
        let db_path = dir.path().join("admin-test.db");
        let db = Arc::new(Database::open(&db_path).unwrap());
        db.init_schema().unwrap();

        let conn = rusqlite::Connection::open(&db_path).unwrap();
        conn.execute("DROP TABLE repo_cache", []).unwrap();
        drop(conn);

        let config = Config {
            server: ServerConfig {
                host: "127.0.0.1".to_owned(),
                port: 0,
            },
            database: DatabaseConfig {
                path: db_path.clone(),
            },
            cache: CacheConfig {
                repo_dir: PathBuf::from(dir.path().join("repos")),
                refresh_ttl_secs: 300,
                refresh_scan_secs: 60,
            },
            upstream: UpstreamConfig {
                github_base: "https://github.com".to_owned(),
            },
        };

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

        (
            AppState {
                config,
                db,
                lifecycle,
            },
            dir,
        )
    }
}
