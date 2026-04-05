use std::path::PathBuf;

use anyhow::bail;

#[derive(Debug, Clone)]
pub struct RepoId {
    owner: String,
    name: String,
}

impl RepoId {
    pub fn new(owner: String, name: String) -> anyhow::Result<Self> {
        validate_component("owner", &owner)?;
        validate_component("repo", &name)?;

        Ok(Self { owner, name })
    }

    pub fn owner(&self) -> &str {
        &self.owner
    }

    pub fn name(&self) -> &str {
        &self.name
    }

    pub fn local_rel_path(&self) -> PathBuf {
        PathBuf::from(&self.owner).join(format!("{}.git", self.name))
    }
}

fn validate_component(label: &str, value: &str) -> anyhow::Result<()> {
    if value.is_empty() {
        bail!("{label} cannot be empty");
    }

    if matches!(value, "." | "..") {
        bail!("{label} cannot be '.' or '..'");
    }

    if value.ends_with(' ') || value.ends_with('.') {
        bail!("{label} cannot end with a space or dot");
    }

    if value.contains('/') || value.contains('\\') {
        bail!("{label} cannot contain path separators");
    }

    if !value.bytes().all(is_allowed_byte) {
        bail!("{label} contains unsupported characters");
    }

    if WINDOWS_RESERVED_NAMES
        .iter()
        .any(|reserved| reserved.eq_ignore_ascii_case(value))
    {
        bail!("{label} uses a reserved Windows file name");
    }

    Ok(())
}

fn is_allowed_byte(byte: u8) -> bool {
    matches!(byte, b'a'..=b'z' | b'A'..=b'Z' | b'0'..=b'9' | b'.' | b'_' | b'-')
}

const WINDOWS_RESERVED_NAMES: &[&str] = &[
    "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8",
    "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
];
