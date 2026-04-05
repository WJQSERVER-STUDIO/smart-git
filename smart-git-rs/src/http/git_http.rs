use axum::{
    body::Bytes,
    extract::{Path, Query, State},
    http::HeaderMap,
    response::Response,
};
use git_server_http::{
    error::AppError,
    handlers::{self, ServiceKind},
};
use serde::Deserialize;

use crate::{app::AppState, repo_id::RepoId};

#[derive(Debug, Deserialize)]
pub struct InfoRefsQuery {
    service: String,
}

pub async fn info_refs(
    State(state): State<AppState>,
    Path((owner, repo)): Path<(String, String)>,
    Query(query): Query<InfoRefsQuery>,
    headers: HeaderMap,
) -> Result<Response, AppError> {
    let repo_id = RepoId::new(owner, repo).map_err(to_bad_request)?;
    if query.service != "git-upload-pack" {
        return Err(AppError::BadRequest(format!(
            "unsupported service: {}",
            query.service
        )));
    }

    let synced = state.lifecycle.sync_for_request(&repo_id).await?;
    handlers::info_refs_endpoint(
        state.lifecycle.git_http_state(),
        &synced.repo_info.relative_path,
        ServiceKind::UploadPack,
        headers,
    )
    .await
}

pub async fn upload_pack(
    State(state): State<AppState>,
    Path((owner, repo)): Path<(String, String)>,
    headers: HeaderMap,
    request: Bytes,
) -> Result<Response, AppError> {
    let repo_id = RepoId::new(owner, repo).map_err(to_bad_request)?;
    let synced = state.lifecycle.sync_for_request(&repo_id).await?;
    handlers::rpc_endpoint(
        state.lifecycle.git_http_state(),
        &synced.repo_info.relative_path,
        ServiceKind::UploadPack,
        headers,
        request,
    )
    .await
}

fn to_bad_request(error: anyhow::Error) -> AppError {
    AppError::BadRequest(error.to_string())
}
