use chrono::NaiveDate;
use clap::Parser;
use std::path::PathBuf;

#[derive(Debug, Parser, Clone)]
#[command(name = "git-bluff")]
#[command(author = "vhxubo")]
#[command(version = "0.2.1")]
#[command(about = "Generate daily reports from git commits", long_about = None)]
pub struct Args {
    #[arg(short, long, default_value = ".")]
    pub directory: PathBuf,

    #[arg(long, default_value = "1")]
    pub depth: usize,

    #[arg(long)]
    pub date: Option<NaiveDate>,

    #[arg(long, default_value = "")]
    pub from: String,

    #[arg(long, default_value = "")]
    pub to: String,

    #[arg(long, value_delimiter = ',', num_args = 1..)]
    pub author: Vec<String>,

    #[arg(short, long, default_value = "false")]
    pub verbose: bool,

    #[arg(long)]
    pub config: Option<PathBuf>,
}
