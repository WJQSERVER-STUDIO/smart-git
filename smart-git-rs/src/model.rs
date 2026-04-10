use serde::Serialize;

#[derive(Debug, Clone, Serialize)]
pub struct RepoCacheRecord {
    pub owner: String,
    pub name: String,
    pub upstream_url: String,
    pub local_path: String,
    pub head_oid: Option<String>,
    pub created_at: i64,
    pub updated_at: i64,
    pub expires_at: i64,
    pub status: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct RepoStatsRecord {
    pub owner: String,
    pub name: String,
    pub clone_count: i64,
    pub request_count: i64,
}

#[derive(Debug, Serialize)]
pub struct ApiSyncResponse {
    pub owner: String,
    pub name: String,
    pub upstream_url: String,
    pub local_path: String,
    pub head_oid: Option<String>,
    pub status: String,
    pub fresh_clone: bool,
    pub refreshed: bool,
}

#[derive(Debug, Serialize)]
pub struct ApiHealthResponse {
    pub status: String,
    pub repo_dir: String,
    pub database_path: String,
    pub github_base: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct ApiRepoRecord {
    pub owner: String,
    pub name: String,
    pub upstream_url: String,
    pub local_path: String,
    pub head_oid: Option<String>,
    pub status: String,
    pub created_at: String,
    pub updated_at: String,
    pub expires_at: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct ApiRepoStats {
    pub owner: String,
    pub name: String,
    pub clone_count: i64,
    pub request_count: i64,
}

#[derive(Debug, Serialize)]
pub struct ApiErrorResponse {
    pub error: String,
}
