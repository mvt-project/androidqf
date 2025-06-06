use serde::{Deserialize, Serialize};

use crate::helper::direntry::DirEntry;
use crate::helper::ErrorsType;

#[derive(Deserialize, Serialize, Debug, Clone, PartialEq)]
pub enum ScandirResult {
    DirEntry(DirEntry),
    Error((String, String)),
}

impl ScandirResult {
    #[inline]
    pub fn digest(&self) -> &String {
        match self {
            Self::DirEntry(e) => &e.digest,
            Self::Error(e) => &e.0,
        }
    }

    #[inline]
    pub fn path(&self) -> &String {
        match self {
            Self::DirEntry(e) => &e.path,
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

    #[inline]
    pub fn is_dir(&self) -> bool {
        match self {
            Self::DirEntry(e) => e.is_dir,
            Self::Error(_) => false,
        }
    }

    #[inline]
    pub fn is_file(&self) -> bool {
        match self {
            Self::DirEntry(e) => e.is_file,
            Self::Error(_) => false,
        }
    }

    #[inline]
    pub fn is_symlink(&self) -> bool {
        match self {
            Self::DirEntry(e) => e.is_symlink,
            Self::Error(_) => false,
        }
    }

    #[inline]
    pub fn ctime(&self) -> f64 {
        match self {
            Self::DirEntry(e) => e.ctime(),
            Self::Error(_) => 0.0,
        }
    }

    #[inline]
    pub fn mtime(&self) -> f64 {
        match self {
            Self::DirEntry(e) => e.mtime(),
            Self::Error(_) => 0.0,
        }
    }

    #[inline]
    pub fn atime(&self) -> f64 {
        match self {
            Self::DirEntry(e) => e.atime(),
            Self::Error(_) => 0.0,
        }
    }

    #[inline]
    pub fn size(&self) -> u64 {
        match self {
            Self::DirEntry(e) => e.st_size,
            Self::Error(_) => 0,
        }
    }

    pub fn to_json(&self) -> serde_json::Result<String> {
        serde_json::to_string(self)
    }
}

#[derive(Deserialize, Serialize, Debug, Clone, PartialEq)]
pub struct ScandirResults {
    pub results: Vec<ScandirResult>,
    pub errors: ErrorsType,
}

impl ScandirResults {
    pub fn new() -> Self {
        ScandirResults {
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

    pub fn extend(&mut self, results: &ScandirResults) {
        self.results.extend_from_slice(&results.results);
        self.errors.extend_from_slice(&results.errors);
    }

    pub fn to_json(&self) -> serde_json::Result<String> {
        serde_json::to_string(self)
    }
}

impl Default for ScandirResults {
    fn default() -> Self {
        Self::new()
    }
}
