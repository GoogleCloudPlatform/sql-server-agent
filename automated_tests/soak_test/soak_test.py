"""Soak tests creation for SQL Server Agent using BCT (Big Cluster Test) framework.

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
from google3.third_party.sqlserveragent.automated_tests.helper import integrationtest_helper
from google3.third_party.sqlserveragent.automated_tests.parameters import sqlserveragent_test_constants

FLAGS = flags.FLAGS


class SoakTest(bigclustertest.TestCase):

  @classmethod
  def setUpClass(cls):
    super().setUpClass()
    vm_win_local = windows_test_utils.CreateWindowsVm(
        cls=cls,
        name='soaktest-win-agent-local',
        raw_names=True,
        wait_for_instance_ready=True,
        vm_image_path=sqlserveragent_test_constants.VM_IMAGE_PREFIX
        + sqlserveragent_test_constants.VM_IMAGE_WIN,
        network_configs=[
            virtual_machine.NetworkConfig(
                network=sqlserveragent_test_constants.NETWORK,
                subnetwork=sqlserveragent_test_constants.SUBNETWORK,
            )
        ],
    )
    vm_win_remote = windows_test_utils.CreateWindowsVm(
        cls=cls,
        name='soaktest-win-agent-remote',
        raw_names=True,
        wait_for_instance_ready=True,
        vm_image_path=sqlserveragent_test_constants.VM_IMAGE_PREFIX
        + sqlserveragent_test_constants.VM_IMAGE_WIN,
        network_configs=[
            virtual_machine.NetworkConfig(
                network=sqlserveragent_test_constants.NETWORK,
                subnetwork=sqlserveragent_test_constants.SUBNETWORK,
            )
        ],
    )
    vm_rhel = cls.CreateVm(
        name='soaktest-rhel-agent',
        raw_names=True,
        wait_for_instance_ready=True,
        vm_image_path=sqlserveragent_test_constants.VM_IMAGE_PREFIX
        + sqlserveragent_test_constants.VM_IMAGE_RHEL,
        network_configs=[
            virtual_machine.NetworkConfig(
                network=sqlserveragent_test_constants.NETWORK,
                subnetwork=sqlserveragent_test_constants.SUBNETWORK,
            )
        ],
    )
    vm_suse = cls.CreateVm(
        name='soaktest-suse-agent',
        raw_names=True,
        wait_for_instance_ready=True,
        vm_image_path=sqlserveragent_test_constants.VM_IMAGE_PREFIX
        + sqlserveragent_test_constants.VM_IMAGE_SUSE,
        network_configs=[
            virtual_machine.NetworkConfig(
                network=sqlserveragent_test_constants.NETWORK,
                subnetwork=sqlserveragent_test_constants.SUBNETWORK,
            )
        ],
    )

    # Set up service account scopes to call workload manager evaluation API
    # See from http://google3/google/cloud/workloadmanager/workloadmanager.yaml
    vm_win_local.AddRobot(
        FLAGS.service_account_name,
        sqlserveragent_test_constants.SERVICE_ACCOUNT_SCOPES,
    )
    vm_win_remote.AddRobot(
        FLAGS.service_account_name,
        sqlserveragent_test_constants.SERVICE_ACCOUNT_SCOPES,
    )
    vm_rhel.AddRobot(
        FLAGS.service_account_name,
        sqlserveragent_test_constants.SERVICE_ACCOUNT_SCOPES,
    )
    vm_suse.AddRobot(
        FLAGS.service_account_name,
        sqlserveragent_test_constants.SERVICE_ACCOUNT_SCOPES,
    )
    # clear the ssh key added to the VM by BCT so the existing key on windows
    # works fine.
    vm_rhel.ClearSshKeys()
    vm_suse.ClearSshKeys()

    # insert VMs
    vm_win_local.Insert()
    vm_win_remote.Insert()
    vm_rhel.Insert()
    vm_suse.Insert()

    # we only do soak test on unstable version.
    cls.integrationtest_helper = integrationtest_helper.IntegrationHelper(
        'unstable',
        vm_win_local,
        vm_win_remote,
        vm_rhel,
        vm_suse,
    )
    time.sleep(300)

  def testSoakTest(self):
    self.integrationtest_helper.SetUpAgent(
        sqlserveragent_test_constants.VM_ID_WIN_LOCAL
    )
    self.integrationtest_helper.SetUpAgent(
        sqlserveragent_test_constants.VM_ID_WIN_REMOTE, 'remote-soak.json'
    )
    self.integrationtest_helper.SetUpAgent(
        sqlserveragent_test_constants.VM_ID_RHEL
    )
    self.integrationtest_helper.SetUpAgent(
        sqlserveragent_test_constants.VM_ID_SUSE
    )

    # let the test run for 10 days.
    for _ in range(10):
      for _ in range(24):
        time.sleep(3600)

if __name__ == '__main__':
  bigclustertest.main()
