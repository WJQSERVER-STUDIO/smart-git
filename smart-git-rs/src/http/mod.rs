mod admin;
mod git_http;

use axum::{
    Router,
    routing::{get, post},
};

use crate::app::AppState;

pub fn build_router(state: AppState) -> Router {
    Router::new()
        .route("/healthz", get(admin::healthz))
        .route("/api/db/data", get(admin::list_cache_records))
        .route("/api/db/sum", get(admin::list_repo_stats))
        .route("/api/cache/:owner/:repo/sync", post(admin::sync_repo))
        .route("/:owner/:repo/info/refs", get(git_http::info_refs))
        .route("/:owner/:repo/git-upload-pack", post(git_http::upload_pack))
        .with_state(state)
}
