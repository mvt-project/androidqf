use serde::{Deserialize, Serialize};

use crate::helper::ErrorsType;

#[derive(Deserialize, Serialize, Debug, Clone)]
pub struct PatternJson {
    pub identifier: String,
    pub offset: usize,
    pub r#match: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub xor_key: Option<u8>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub plaintext: Option<String>,
}

#[derive(Deserialize, Serialize, Debug, Clone)]
pub struct RuleJson {
    pub identifier: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub namespace: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub meta: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tags: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub strings: Option<Vec<PatternJson>>,
}

#[derive(Deserialize, Serialize, Debug, Clone)]
pub struct YaraEntry {
    pub path: String,
    pub count: usize,
    pub rules: Vec<RuleJson>,
}

#[derive(Deserialize, Serialize, Debug, Clone)]
pub enum ScanResult {
    YaraEntry(YaraEntry),
    Error((String, String)),
}

impl ScanResult {
    #[inline]
    pub fn path(&self) -> &String {
        match self {
            Self::YaraEntry(e) => &e.path,
            Self::Error(e) => &e.0,
        }
    }

    #[inline]
    pub fn error(&self) -> Option<&(String, String)> {
        match self {
            Self::Error(e) => Some(e),
            _ => None,
        }
    }

    pub fn to_json(&self) -> serde_json::Result<String> {
        serde_json::to_string(self)
    }
}

#[derive(Deserialize, Serialize, Debug, Clone)]
pub struct ScanResults {
    pub results: Vec<ScanResult>,
    pub errors: ErrorsType,
}

impl ScanResults {
    pub fn new() -> Self {
        ScanResults {
            results: Vec::new(),
            errors: Vec::new(),
        }
    }

    pub fn clear(&mut self) {
        self.results.clear();
        self.errors.clear();
    }

    #[inline]
    pub fn is_empty(&self) -> bool {
        self.results.is_empty() && self.errors.is_empty()
    }

    #[inline]
    pub fn len(&self) -> usize {
        self.results.len() + self.errors.len()
    }

    pub fn extend(&mut self, results: &ScanResults) {
        self.results.extend_from_slice(&results.results);
        self.errors.extend_from_slice(&results.errors);
    }

    pub fn to_json(&self) -> serde_json::Result<String> {
        serde_json::to_string(self)
    }
}

impl Default for ScanResults {
    fn default() -> Self {
        Self::new()
    }
}
