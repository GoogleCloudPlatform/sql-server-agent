"""Load tests creation for SQL Server Agent using BCT (Big Cluster Test) framework.

This class will handle the creation of BCT test cases and associated VMs.

The service account must have IAM permissions for any calls necessary for
running the tests in the VM.
"""

import time

import google3

from absl import flags

from google3.cloud.cluster.testing.framework import bigclustertest
from google3.cloud.cluster.testing.framework import virtual_machine
from google3.cloud.cluster.testing.framework import windows_test_utils
from google3.third_party.sqlserveragent.automated_tests.helper import agent_helper
from google3.third_party.sqlserveragent.automated_tests.helper import loadtest_helper
from google3.third_party.sqlserveragent.automated_tests.parameters import sqlserveragent_test_constants

FLAGS = flags.FLAGS
IMAGE_PATH_PREFIX = 'projects/wlm-for-sqlserver-integration/images/loadtest-'
SQL_VERSION = 'sql2022'
HAMMERDB_VERSION = 'hammerdb-4-9'


class LoadTest(bigclustertest.TestCase):
  """Load test class."""

  @classmethod
  def setUpClass(cls):
    super().setUpClass()
    vm_sqlserver = windows_test_utils.CreateWindowsVm(
        cls=cls,
        name='loadtest-sqlserver',
        raw_names=True,
        wait_for_instance_ready=True,
        vm_image_path=f'{IMAGE_PATH_PREFIX}{SQL_VERSION}',
        network_configs=[
            virtual_machine.NetworkConfig(
                network=sqlserveragent_test_constants.NETWORK,
                subnetwork=sqlserveragent_test_constants.SUBNETWORK,
            )
        ],
    )
    cls.vm_sqlserver = vm_sqlserver
    vm_sqlserver.AddRobot(
        FLAGS.service_account_name,
        sqlserveragent_test_constants.SERVICE_ACCOUNT_SCOPES,
    )
    vm_sqlserver.Insert()
    # We need to wait for 1 min so the VM is fully booted and then we are able
    # to get the private ip of the VM.
    time.sleep(60)

    vm_hammerdb = windows_test_utils.CreateWindowsVm(
        cls=cls,
        name='loadtest-hammerdb-instance',
        raw_names=True,
        wait_for_instance_ready=True,
        vm_image_path=f'{IMAGE_PATH_PREFIX}{HAMMERDB_VERSION}',
        network_configs=[
            virtual_machine.NetworkConfig(
                network=sqlserveragent_test_constants.NETWORK,
                subnetwork=sqlserveragent_test_constants.SUBNETWORK,
            )
        ],
    )
    vm_hammerdb.AddRobot(
        FLAGS.service_account_name,
        sqlserveragent_test_constants.SERVICE_ACCOUNT_SCOPES,
    )
    vm_hammerdb.Insert()
    # convert version from format '1-1' to '1.1'
    hdm_version = HAMMERDB_VERSION.split('-', 1)[1].replace('-', '.')
    cls.lt_helper = loadtest_helper.LoadtestHelper(
        vm_sqlserver,
        vm_hammerdb,
        hdm_version,
    )
    cls.lt_helper.SetupHammerDB()
    cls.lt_helper.RunSqlScripts('runtime_collection.sql')
    cls.lt_helper.RunSqlScripts('sp_write_performance_counters.sql')

  def testLoadTest(self):
    self.lt_helper.RunSqlScripts('run_loadtest.sql', background=True)
    self.lt_helper.RunVirtualUsers()
    self.lt_helper.StopBackgroundJob()
    self.lt_helper.UploadLoadTestResultToBucket()
    self.lt_helper.ProcessLoadTestResult()

  def testLoadTestWithAgentInstalled(self):
    ah = agent_helper.AgentHelper(self.vm_sqlserver, True, 'unstable')
    ah.AddRepo()
    ah.InstallAgent()
    ah.UpdateConfiguration('local.json')
    ah.StartService()
    ah.VerifyAgentRunning()
    self.lt_helper.RunSqlScripts('run_loadtest.sql', background=True)
    self.lt_helper.RunVirtualUsers()
    self.lt_helper.StopBackgroundJob()
    self.lt_helper.UploadLoadTestResultToBucket(agent_installed=True)
    self.lt_helper.ProcessLoadTestResult(agent_installed=True)

  def testTestResult(self):
    self.lt_helper.GenerateLoadTestResult()


if __name__ == '__main__':
  bigclustertest.main()
