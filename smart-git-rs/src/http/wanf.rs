use std::io;

use axum::{
    body::Body,
    http::{header, HeaderValue, Response, StatusCode},
};
use wanf::value::{ObjectKind, ObjectValue, Value};
use wanf::{OutputStyle, StreamEncoder};

use crate::model::{
    ApiErrorResponse, ApiHealthResponse, ApiRepoRecord, ApiRepoStats, ApiSyncResponse,
};

pub fn health_response(status: StatusCode, value: &ApiHealthResponse) -> Response<Body> {
    wanf_response(status, api_health_to_value(value))
}

pub fn repo_records_response(status: StatusCode, values: &[ApiRepoRecord]) -> Response<Body> {
    wanf_response(
        status,
        object_value(vec![(
            "items",
            list_to_value(values.iter().map(api_repo_to_value).collect()),
        )]),
    )
}

pub fn repo_stats_response(status: StatusCode, values: &[ApiRepoStats]) -> Response<Body> {
    wanf_response(
        status,
        object_value(vec![(
            "items",
            list_to_value(values.iter().map(api_stats_to_value).collect()),
        )]),
    )
}

pub fn sync_response(status: StatusCode, value: &ApiSyncResponse) -> Response<Body> {
    wanf_response(status, api_sync_to_value(value))
}

pub fn error_response(status: StatusCode, message: impl Into<String>) -> Response<Body> {
    wanf_response(
        status,
        object_value(vec![(
            "error",
            Value::String(
                ApiErrorResponse {
                    error: message.into(),
                }
                .error,
            ),
        )]),
    )
}

fn wanf_response(status: StatusCode, value: Value) -> Response<Body> {
    let mut body = Vec::new();
    let encoder = StreamEncoder::new(&mut body).with_style(OutputStyle::Block);
    if let Err(error) = encoder.encode(&value) {
        return plain_error_response(StatusCode::INTERNAL_SERVER_ERROR, error.to_string());
    }

    let mut response = Response::new(Body::from(body));
    *response.status_mut() = status;
    response.headers_mut().insert(
        header::CONTENT_TYPE,
        HeaderValue::from_static("application/vnd.wjqserver.wanf; charset=utf-8"),
    );
    response.headers_mut().insert(
        header::X_CONTENT_TYPE_OPTIONS,
        HeaderValue::from_static("nosniff"),
    );
    response
}

fn plain_error_response(status: StatusCode, message: String) -> Response<Body> {
    let mut response = Response::new(Body::from(message));
    *response.status_mut() = status;
    response.headers_mut().insert(
        header::CONTENT_TYPE,
        HeaderValue::from_static("text/plain; charset=utf-8"),
    );
    response
}

fn api_health_to_value(value: &ApiHealthResponse) -> Value {
    object_value(vec![
        ("status", Value::String(value.status.clone())),
        ("repo_dir", Value::String(value.repo_dir.clone())),
        ("database_path", Value::String(value.database_path.clone())),
        ("github_base", Value::String(value.github_base.clone())),
    ])
}

fn api_repo_to_value(value: &ApiRepoRecord) -> Value {
    object_value(vec![
        ("owner", Value::String(value.owner.clone())),
        ("name", Value::String(value.name.clone())),
        ("upstream_url", Value::String(value.upstream_url.clone())),
        ("local_path", Value::String(value.local_path.clone())),
        ("head_oid", option_string_value(value.head_oid.clone())),
        ("status", Value::String(value.status.clone())),
        ("created_at", Value::String(value.created_at.clone())),
        ("updated_at", Value::String(value.updated_at.clone())),
        ("expires_at", Value::String(value.expires_at.clone())),
    ])
}

fn api_stats_to_value(value: &ApiRepoStats) -> Value {
    object_value(vec![
        ("owner", Value::String(value.owner.clone())),
        ("name", Value::String(value.name.clone())),
        ("clone_count", Value::Int(value.clone_count)),
        ("request_count", Value::Int(value.request_count)),
    ])
}

fn api_sync_to_value(value: &ApiSyncResponse) -> Value {
    object_value(vec![
        ("owner", Value::String(value.owner.clone())),
        ("name", Value::String(value.name.clone())),
        ("upstream_url", Value::String(value.upstream_url.clone())),
        ("local_path", Value::String(value.local_path.clone())),
        ("head_oid", option_string_value(value.head_oid.clone())),
        ("status", Value::String(value.status.clone())),
        ("fresh_clone", Value::Bool(value.fresh_clone)),
        ("refreshed", Value::Bool(value.refreshed)),
    ])
}

fn list_to_value(values: Vec<Value>) -> Value {
    Value::List(values)
}

fn option_string_value(value: Option<String>) -> Value {
    match value {
        Some(value) => Value::String(value),
        None => Value::Null,
    }
}

fn object_value(entries: Vec<(&str, Value)>) -> Value {
    let mut object = ObjectValue::new(ObjectKind::Root);
    for (key, value) in entries {
        object.entries.insert(String::from(key), value);
    }
    Value::Object(object)
}

#[allow(dead_code)]
fn _assert_send_sync() {
    fn assert_send_sync<T: Send + Sync>() {}
    assert_send_sync::<io::Cursor<Vec<u8>>>();
}
