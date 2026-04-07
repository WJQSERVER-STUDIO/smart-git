use axum::{
    extract::{Path, State},
    http::StatusCode,
    response::{IntoResponse, Response},
};

use crate::{
    app::AppState,
    model::{ApiHealthResponse, ApiRepoRecord, ApiRepoStats, ApiSyncResponse},
    repo_id::RepoId,
};

use gitserver_http::error::AppError as GitHttpAppError;

use super::wanf;

pub async fn healthz(State(state): State<AppState>) -> Response {
    wanf::health_response(
        StatusCode::OK,
        &ApiHealthResponse {
            status: "ok".to_owned(),
            repo_dir: state.config.cache.repo_dir.display().to_string(),
            database_path: state.config.database.path.display().to_string(),
            github_base: state.config.upstream.github_base.clone(),
        },
    )
}

pub async fn list_cache_records(State(state): State<AppState>) -> ApiResult<Response> {
    let records = state
        .db
        .list_cache_records()?
        .into_iter()
        .map(|record| ApiRepoRecord {
            owner: record.owner,
            name: record.name,
            upstream_url: record.upstream_url,
            local_path: record.local_path,
            head_oid: record.head_oid,
            status: record.status,
            created_at: timestamp_to_rfc3339(record.created_at),
            updated_at: timestamp_to_rfc3339(record.updated_at),
            expires_at: timestamp_to_rfc3339(record.expires_at),
        })
        .collect::<Vec<_>>();
    Ok(wanf::repo_records_response(StatusCode::OK, &records))
}

pub async fn list_repo_stats(State(state): State<AppState>) -> ApiResult<Response> {
    let stats = state
        .db
        .list_stats()?
        .into_iter()
        .map(|record| ApiRepoStats {
            owner: record.owner,
            name: record.name,
            clone_count: record.clone_count,
            request_count: record.request_count,
        })
        .collect::<Vec<_>>();
    Ok(wanf::repo_stats_response(StatusCode::OK, &stats))
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

    Ok(wanf::sync_response(
        status,
        &ApiSyncResponse {
            owner: repo_id.owner().to_owned(),
            name: repo_id.name().to_owned(),
            upstream_url: synced.record.upstream_url,
            local_path: synced.record.local_path,
            head_oid: synced.record.head_oid,
            status: synced.record.status,
            fresh_clone: synced.fresh_clone,
            refreshed: synced.refreshed,
        },
    ))
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
        wanf::error_response(StatusCode::BAD_REQUEST, self.0.to_string())
    }
}

fn api_error_from_git(error: GitHttpAppError) -> ApiError {
    let message = match error {
        GitHttpAppError::NotFound(message)
        | GitHttpAppError::BadRequest(message)
        | GitHttpAppError::Internal(message) => message,
        GitHttpAppError::Unauthorized => "authentication required".to_owned(),
    };
    ApiError(anyhow::anyhow!(message))
}

fn timestamp_to_rfc3339(value: i64) -> String {
    if value <= 0 {
        return String::new();
    }

    use std::time::{Duration, UNIX_EPOCH};

    let time = UNIX_EPOCH + Duration::from_secs(value as u64);
    let datetime = time::OffsetDateTime::from(time);
    datetime
        .format(&time::format_description::well_known::Rfc3339)
        .unwrap_or_default()
}
