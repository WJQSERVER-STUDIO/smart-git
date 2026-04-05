mod app;
mod config;
mod db;
mod git;
mod http;
mod lifecycle;
mod model;
mod repo_id;

use std::path::PathBuf;

use clap::Parser;
use tracing_subscriber::{EnvFilter, fmt};

use crate::config::Config;

#[derive(Debug, Parser)]
#[command(name = "smart-git-rs")]
#[command(about = "Container-first GitHub mirror service rewrite")]
struct Cli {
    #[arg(short = 'c', long = "config", value_name = "FILE")]
    config: Option<PathBuf>,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    init_tracing();

    let cli = Cli::parse();
    let config = Config::load(cli.config.as_deref())?;

    app::run(config).await
}

fn init_tracing() {
    let filter = EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info"));
    fmt()
        .with_env_filter(filter)
        .with_target(false)
        .compact()
        .init();
}
