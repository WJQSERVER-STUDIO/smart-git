use axum::{
    Json,
    extract::{Path, State},
    http::StatusCode,
    response::{IntoResponse, Response},
};
use serde_json::json;

use crate::{
    app::AppState,
    model::{HealthResponse, RepoCacheRecord, SyncResponse},
    repo_id::RepoId,
};

pub async fn healthz(State(state): State<AppState>) -> Json<HealthResponse> {
    Json(HealthResponse {
        status: "ok",
        repo_dir: state.config.cache.repo_dir.display().to_string(),
        database_path: state.config.database.path.display().to_string(),
        github_base: state.config.upstream.github_base.clone(),
    })
}

pub async fn list_cache_records(
    State(state): State<AppState>,
) -> ApiResult<Json<Vec<RepoCacheRecord>>> {
    Ok(Json(state.db.list_cache_records()?))
}

pub async fn list_repo_stats(State(state): State<AppState>) -> ApiResult<Response> {
    let stats = state.db.list_stats()?;
    Ok((StatusCode::OK, Json(stats)).into_response())
}

pub async fn sync_repo(
    State(state): State<AppState>,
    Path((owner, repo)): Path<(String, String)>,
) -> ApiResult<Response> {
    let repo_id = RepoId::new(owner, repo)?;
    let synced = state
        .lifecycle
        .sync_manual(&repo_id)
        .await
        .map_err(api_error_from_git)?;

    let status = if synced.fresh_clone {
        StatusCode::CREATED
    } else {
        StatusCode::OK
    };

    Ok((
        status,
        Json(SyncResponse {
            owner: repo_id.owner().to_owned(),
            name: repo_id.name().to_owned(),
            upstream_url: synced.record.upstream_url,
            local_path: synced.record.local_path,
            head_oid: synced.record.head_oid,
            fresh_clone: synced.fresh_clone,
            refreshed: synced.refreshed,
        }),
    )
        .into_response())
}

type ApiResult<T> = Result<T, ApiError>;

pub(crate) struct ApiError(anyhow::Error);

impl<E> From<E> for ApiError
where
    E: Into<anyhow::Error>,
{
    fn from(error: E) -> Self {
        Self(error.into())
    }
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        (
            StatusCode::BAD_REQUEST,
            Json(json!({
                "error": self.0.to_string(),
            })),
        )
            .into_response()
    }
}

fn api_error_from_git(error: git_server_http::error::AppError) -> ApiError {
    let message = match error {
        git_server_http::error::AppError::NotFound(message)
        | git_server_http::error::AppError::BadRequest(message)
        | git_server_http::error::AppError::Internal(message) => message,
        git_server_http::error::AppError::Unauthorized => "authentication required".to_owned(),
    };
    ApiError(anyhow::anyhow!(message))
}
