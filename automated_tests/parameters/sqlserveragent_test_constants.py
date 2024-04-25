"""Module for SQL Server Agent automated tests constants."""

import enum

from google3.cloud.cluster.testing.framework import enums


class TestFrequency(enum.Enum):
  HOURLY = "HOURLY"
  DAILY = "DAILY"

VM_IMAGE_PREFIX = "projects/wlm-for-sqlserver-integration/images/"
VM_IMAGE_WIN = "win19-sql22-v20240425"
VM_IMAGE_RHEL = "rhel8-sql2022-v20240417"
VM_IMAGE_SUSE = "suse15-sql2022-v20240424"
VM_ID_WIN_LOCAL = 0
VM_ID_WIN_REMOTE = 1
VM_ID_RHEL = 2
VM_ID_SUSE = 3
NETWORK = "projects/wlm-for-sqlserver-integration/global/networks/wlm4sql-vpc"
SUBNETWORK = "projects/wlm-for-sqlserver-integration/regions/us-central1/subnetworks/wlm4sql-subnet-0"
WINDOWS_IDENTIFIER_STRING = "windows"
LINUX_IDENTIFIER_STRING = "linux"
SERVICE_ACCOUNT_SCOPES = [
    "https://www.googleapis.com/auth/devstorage.read_write",
    "https://www.googleapis.com/auth/cloud-platform",
]
PD_TYPE_LIST = list(map(lambda pd: pd.value, enums.PdProductType))
VALID_REPO_VALUES = ["none", "stable", "unstable", "google3"]
SQL_SERVER_AGENT_NAME = "google-cloud-sql-server-agent"
SQL_SERVER_AGENT_EXE_PATH = (
    "'C:\\Program"
    " Files\\Google\\google-cloud-sql-server-agent\\google-cloud-sql-server-agent.exe'"
)
SQL_SERVER_AGENT_CONFIG_PATH_WINDOWS = (
    "'C:\\Program"
    " Files\\Google\\google-cloud-sql-server-agent\\configuration.json'"
)
SQL_SERVER_AGENT_CONFIG_PATH_LINUX = "/etc/google-cloud-sql-server-agent/configuration.json"
SQL_SERVER_AGENT_LOG_PATH_WINDOWS = "C:\\ProgramData\\Google\\google-cloud-sql-server-agent\\logs\\google-cloud-sql-server-agent.log"
SQL_SERVER_AGENT_LOG_PATH_LINUX = "/var/log/google-cloud-sql-server-agent.log"

AGENT_FOR_SQL_SERVER_AGENT_DIR = (
    "google3/third_party/sqlserveragent/automated_tests/"
)
