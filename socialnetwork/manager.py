#!/usr/bin/env python3
import argparse
import googleapiclient.discovery
from google.oauth2 import service_account
import sys
from plumbum import FG
import toml
from time import sleep
from tqdm import tqdm
import datetime
import os
import socket

APP_PORT                    = 9000
NUM_DOCKER_SWARM_SERVICES   = 20
NUM_DOCKER_SWARM_NODES      = 3
BASE_DIR                    = os.path.dirname(os.path.realpath(__file__))

# -----------
# GCP profile
# -----------
# TBD
GCP_PROJECT_ID                  = None
GCP_USERNAME                    = None
GCP_CREDENTIALS                 = None
GCP_COMPUTE                     = None

# ---------------------
# GCP app configuration
# ---------------------
# same as in terraform
APP_FOLDER_NAME           = "socialnetwork"
GCP_INSTANCE_APP_WRK2     = "weaver-dsb-app-wrk2"
GCP_INSTANCE_APP_EU       = "weaver-dsb-app-eu"
GCP_INSTANCE_APP_US       = "weaver-dsb-app-us"
GCP_INSTANCE_DB_MANAGER   = "weaver-dsb-db-manager"
GCP_INSTANCE_DB_EU        = "weaver-dsb-db-eu"
GCP_INSTANCE_DB_US        = "weaver-dsb-db-us"
GCP_ZONE_MANAGER          = "europe-west3-a"
GCP_ZONE_EU               = "europe-west3-a"
GCP_ZONE_US               = "us-central1-a"

# --------------------
# Helpers
# --------------------

def load_gcp_profile():
  import yaml
  global GCP_PROJECT_ID, GCP_USERNAME, GCP_COMPUTE
  try:
    with open('gcp/config.yml', 'r') as file:
      config = yaml.safe_load(file)
      GCP_PROJECT_ID  = str(config['project_id'])
      GCP_USERNAME    = str(config['username'])
    GCP_CREDENTIALS   = service_account.Credentials.from_service_account_file("gcp/credentials.json")
    GCP_COMPUTE = googleapiclient.discovery.build('compute', 'v1', credentials=GCP_CREDENTIALS)
  except Exception as e:
      print(f"[ERROR] error loading gcp profile: {e}")
      exit(-1)

def display_progress_bar(duration, info_message):
  print(f"[INFO] {info_message} for {duration} seconds...")
  for _ in tqdm(range(int(duration))):
    sleep(1)

def get_instance_host(instance_name, zone):
  instance = GCP_COMPUTE.instances().get(project=GCP_PROJECT_ID, zone=zone, instance=instance_name).execute()
  network_interface = instance['networkInterfaces'][0]
  # public, private
  return network_interface['accessConfigs'][0]['natIP']

def is_port_open(address, port):
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    result = sock.connect_ex((address, port))
    sock.close()
    return result == 0

def run_workload(timestamp, deployment, url, threads, conns, duration, rate):
  import threading

  # verify workload files
  if not os.path.exists(f"{BASE_DIR}/wrk2/wrk"):
    print(f"[ERROR] error running workload: '{BASE_DIR}/wrk2/wrk' file does not exist")
    exit(-1)

  progress_thread = threading.Thread(target=display_progress_bar, args=(duration, "running workload",))
  progress_thread.start()

  from plumbum import local
  with local.env(HOST_EU=url, HOST_US=url):
    wrk2 = local['./wrk2/wrk']
    output = wrk2['-D', 'exp', '-t', str(threads), '-c', str(conns), '-d', str(duration), '-L', '-s', './wrk2/scripts/social-network/compose-post.lua', f'{url}/wrk2-api/post/compose', '-R', str(rate)]()

    filepath = f"evaluation/{deployment}/{timestamp}/workload.out"
    os.makedirs(os.path.dirname(filepath), exist_ok=True)
    with open(filepath, "w") as f:
      f.write(output)

    print(output)
    print(f"[INFO] workload results saved at {filepath}")

  progress_thread.join()
  return output

def gen_weaver_config_gcp():
  host_eu = get_instance_host(GCP_INSTANCE_DB_EU, GCP_ZONE_EU)
  host_us = get_instance_host(GCP_INSTANCE_DB_US, GCP_ZONE_US)

  data = toml.load("deploy/weaver/weaver-template-eu.toml")

  # europe
  for _, config in data.items():
    if 'mongodb_address' in config:
      config['mongodb_address'] = host_eu
    if 'redis_address' in config:
      config['redis_address'] = host_eu
    if 'rabbitmq_address' in config:
      config['rabbitmq_address'] = host_eu
    if 'memcached_address' in config:
      config['memcached_address'] = host_eu
  filepath_eu = "deploy/tmp/weaver-gcp-eu.toml"
  f = open(filepath_eu,'w')
  toml.dump(data, f)
  f.close()

  # us
  data = toml.load("deploy/weaver/weaver-template-us.toml")
  for _, config in data.items():
    if 'mongodb_address' in config:
      config['mongodb_address'] = host_us
    if 'redis_address' in config:
      config['redis_address'] = host_us
    if 'rabbitmq_address' in config:
      config['rabbitmq_address'] = host_us
    if 'memcached_address' in config:
      config['memcached_address'] = host_us
  filepath_us = "deploy/tmp/weaver-gcp-us.toml"
  f = open(filepath_us,'w')
  toml.dump(data, f)
  f.close()

  print(f"[INFO] generated app config for GCP at {filepath_eu} and {filepath_us}")

def gen_ansible_vars(workload_timestamp=None, deployment_type=None):
  import yaml

  with open('deploy/ansible/vars.yml', 'r') as file:
    data = yaml.safe_load(file)

  data['base_dir'] = BASE_DIR
  data['workload_timestamp'] = workload_timestamp if workload_timestamp else None
  data['deployment_type'] = deployment_type if deployment_type else None

  with open('deploy/tmp/ansible-vars.yml', 'w') as file:
    yaml.dump(data, file)

def gen_ansible_inventory_gcp():
  from jinja2 import Environment
  import textwrap

  # datastores
  host_db_manager = get_instance_host(GCP_INSTANCE_DB_MANAGER, GCP_ZONE_MANAGER)
  host_db_eu      = get_instance_host(GCP_INSTANCE_DB_EU, GCP_ZONE_EU)
  host_db_us      = get_instance_host(GCP_INSTANCE_DB_US, GCP_ZONE_US)
  # app
  host_app_wrk2   = get_instance_host(GCP_INSTANCE_APP_WRK2, GCP_ZONE_MANAGER)
  host_app_eu     = get_instance_host(GCP_INSTANCE_APP_EU, GCP_ZONE_EU)
  host_app_us     = get_instance_host(GCP_INSTANCE_APP_US, GCP_ZONE_US)

  template = """
    [swarm_manager]
    weaver-dsb-db-manager ansible_host={{ host_db_manager }} user=mafaldacf

    [swarm_workers]
    weaver-dsb-db-eu      ansible_host={{ host_db_eu }} user=mafaldacf
    weaver-dsb-db-us      ansible_host={{ host_db_us }} user=mafaldacf

    [app_wrk2]
    weaver-dsb-app-wrk2   ansible_host={{ host_app_wrk2 }} user=mafaldacf

    [app_services]
    weaver-dsb-app-eu     ansible_host={{ host_app_eu }} user=mafaldacf region="eu" app_port="9000"
    weaver-dsb-app-us     ansible_host={{ host_app_us }} user=mafaldacf region="us" app_port="9001"
  """
  inventory = Environment().from_string(template).render({
    'host_db_manager':  host_db_manager,
    'host_db_eu':       host_db_eu,
    'host_db_us':       host_db_us,
    'host_app_wrk2':    host_app_wrk2,
    'host_app_eu':      host_app_eu,
    'host_app_us':      host_app_us,
  })

  filename = "deploy/tmp/ansible-inventory-gcp.cfg"
  with open(filename, 'w') as f:
    f.write(textwrap.dedent(inventory))
  print(f"[INFO] generated ansible inventory for GCP at '{filename}'")

# METRICS FORMAT
#╭────────────────────────────────────────────────────────────────────────╮
#│ // The number of composed posts                                        │
#│ composed_posts: COUNTER                                                │
#├───────────────────┬────────────────────┬───────────────────────┬───────┤
#│ serviceweaver_app │ serviceweaver_node │ serviceweaver_version │ Value │
#├───────────────────┼────────────────────┼───────────────────────┼───────┤
#│ weaver-dsb-db     │ 0932683b           │ 1cd20361              │ 0     │
#│ weaver-dsb-db     │ 1205179c           │ 1cd20361              │ 0     │
#|  ...              | ...                | ...                   | ...   |
#╰───────────────────┴────────────────────┴───────────────────────┴───────╯
#
#╭────────────────────────────────────────────────────────────────────────╮
#│ // The number of times an cross-service inconsistency has occured      │
#│ inconsistencies: COUNTER                                               │
#├───────────────────┬────────────────────┬───────────────────────┬───────┤
#│ serviceweaver_app │ serviceweaver_node │ serviceweaver_version │ Value │
#├───────────────────┼────────────────────┼───────────────────────┼───────┤
#│ weaver-dsb-db     │ 0932683b           │ 1cd20361              │ 0     │
#│ weaver-dsb-db     │ 1205179c           │ 1cd20361              │ 0     │
#|  ...              | ...                | ...                   | ...   |
#╰───────────────────┴────────────────────┴───────────────────────┴───────╯

# Possible commands to retrieve metrics:
# weaver multi metrics sn_compose_post_duration_ms
# weaver multi metrics sn_composed_posts
# weaver multi metrics sn_write_post_duration_ms
# weaver multi metrics sn_queue_duration_ms
# weaver multi metrics sn_received_notifications
# weaver multi metrics sn_inconsistencies
def metrics(deployment, timestamp=None):
  import yaml
  from plumbum.cmd import weaver
  import re

  pattern = re.compile(r'^.*│.*│.*│.*│\s*(\d+\.?\d*)\s*│.*$', re.MULTILINE)

  def get_filter_metrics(metric_name):
    return weaver['multi', 'metrics', metric_name]()

  # Steps:
  # 1. get desired metrics that will be listed for each process using get_filter_metrics
  # 2. then average the values of all processes which is ok since weaver metrics are limited to averages

  # wkr2 api
  compose_post_duration_metrics = get_filter_metrics('sn_compose_post_duration_ms')
  compose_post_duration_metrics_values = pattern.findall(compose_post_duration_metrics)
  compose_post_duration_avg_ms = sum(float(value) for value in compose_post_duration_metrics_values)/len(compose_post_duration_metrics_values) if compose_post_duration_metrics_values else 0
  # compose post service
  composed_posts_metrics = get_filter_metrics('sn_composed_posts')
  composed_posts_count = sum(int(value) for value in pattern.findall(composed_posts_metrics))
  # post storage service
  write_post_duration_metrics = get_filter_metrics('sn_write_post_duration_ms')
  write_post_duration_metrics_values = pattern.findall(write_post_duration_metrics)
  write_post_duration_avg_ms = sum(float(value) for value in write_post_duration_metrics_values)/len(write_post_duration_metrics_values) if write_post_duration_metrics_values else 0
  # write home timeline service
  queue_duration_metrics = get_filter_metrics('sn_queue_duration_ms')
  queue_duration_metrics_values = pattern.findall(queue_duration_metrics)
  queue_duration_avg_ms = sum(float(value) for value in queue_duration_metrics_values)/len(queue_duration_metrics_values) if queue_duration_metrics_values else 0
  received_notifications_metrics = get_filter_metrics('sn_received_notifications')
  received_notifications_count = sum(int(value) for value in pattern.findall(received_notifications_metrics))
  inconsitencies_metrics = get_filter_metrics('sn_inconsistencies')
  inconsistencies_count = sum(int(value) for value in pattern.findall(inconsitencies_metrics))
  
  pc_inconsistencies = "{:.2f}".format((inconsistencies_count / received_notifications_count) * 100) if received_notifications_count != 0 else 0
  #pc_received_notifications = "{:.2f}".format((received_notifications_count / composed_posts_count) * 100) if composed_posts_count else 0

  compose_post_duration_avg_ms = "{:.2f}".format(compose_post_duration_avg_ms)
  write_post_duration_avg_ms = "{:.2f}".format(write_post_duration_avg_ms)
  queue_duration_avg_ms = "{:.2f}".format(queue_duration_avg_ms)

  results = {
    'num_composed_posts': int(composed_posts_count),
    'num_received_notifications': int(received_notifications_count),
    'num_inconsistencies': int(inconsistencies_count),
    'per_inconsistencies': float(pc_inconsistencies),
    'avg_compose_post_duration_ms': float(compose_post_duration_avg_ms),
    'avg_write_post_duration_msg': float(write_post_duration_avg_ms),
    'avg_queue_duration_ms': float(queue_duration_avg_ms),
  }

  # save file if we ran workload
  if timestamp:
    filepath = f"evaluation/{deployment}/{timestamp}/metrics.yml"
    os.makedirs(os.path.dirname(filepath), exist_ok=True)
    with open(filepath, 'w') as outfile:
      yaml.dump(results, outfile, default_flow_style=False)
    print(yaml.dump(results, default_flow_style=False))
    print(f"[INFO] evaluation results saved at {filepath}")

# --------------------
# GCP
# --------------------

def gcp_configure():
  from plumbum.cmd import gcloud

  try:
    print("[INFO] configuring firewalls")
    # weaver-dsb-socialnetwork:
    # tcp ports: 9000,9001
    # weaver-dsb-storage:
    # tcp ports: 27017,27018,15672,15673,5672,5673,6381,6382,6383,6384,6385,6386,6387,6388,11212,11213,11214,11215,11216,11217
    # weaver-dsb-swarm:
    # tcp ports: 2376,2377,7946
    # udp ports: 4789,7946
    firewalls = {
      'weaver-dsb-socialnetwork': 'tcp:9000,tcp:9001',
      'weaver-dsb-storage': 'tcp:27017,tcp:27018,tcp:15672,tcp:15673,tcp:5672,tcp:5673,tcp:6381,tcp:6382,tcp:6383,tcp:6384,tcp:6385,tcp:6386,tcp:6387,tcp:6388,tcp:11212,tcp:11213,tcp:11214,tcp:11215,tcp:11216,tcp:11217',
      'weaver-dsb-swarm': 'tcp:2376,tcp:2377,tcp:7946,udp:4789,udp:7946'
    }

    for name, rules in firewalls.items():
      gcloud['compute', 
            '--project', GCP_PROJECT_ID, 'firewall-rules', 'create', 
            f'{name}',
            '--direction=INGRESS',
            '--priority=100',
            '--network=default',
            '--action=ALLOW',
            f'--rules={rules}',
            '--source-ranges=0.0.0.0/0'] & FG
  except Exception as e:
    print(f"[ERROR] could not configure firewalls: {e}\n\n")

def gcp_deploy():
  from plumbum.cmd import terraform, cp, ansible_playbook

  terraform['-chdir=./deploy/terraform', 'init'] & FG
  terraform['-chdir=./deploy/terraform', 'apply'] & FG

  display_progress_bar(30, "waiting for all machines to initialize")

  cp["deploy/ansible/ansible.cfg", os.path.expanduser("~/.ansible.cfg")] & FG
  print("[INFO] copied deploy/ansible/ansible.cfg to ~.ansible.cfg")

  # generate temporary files for this deployment
  os.makedirs("deploy/tmp", exist_ok=True)
  print(f"[INFO] created {BASE_DIR}/deploy/tmp/ directory")
  # generate weaver config with hosts of datastores in gcp machines
  gen_weaver_config_gcp()
  # generate ansible inventory with hosts of all gcp machines
  gen_ansible_inventory_gcp()
  # generate ansible inventory with extra variables for current deployment
  gen_ansible_vars()
  
  ansible_playbook["deploy/ansible/playbooks/install-machines.yml", "-i", "deploy/tmp/ansible-inventory-gcp.cfg", "--extra-vars", "@deploy/tmp/ansible-vars.yml"] & FG

def gcp_info():
  from plumbum.cmd import gcloud
  gcloud['compute', 'ssh', GCP_INSTANCE_DB_MANAGER, '--command', 'sudo docker node ls'] & FG
  gcloud['compute', 'ssh', GCP_INSTANCE_DB_MANAGER, '--command', 'sudo docker service ls'] & FG

  print("\n--- DATASTORES ---")
  print("storage manager running @", get_instance_host(GCP_INSTANCE_DB_MANAGER, GCP_ZONE_MANAGER))
  print(f"storage in {GCP_ZONE_EU} running @", get_instance_host(GCP_INSTANCE_DB_EU, GCP_ZONE_EU))
  print(f"storage in {GCP_ZONE_US} running @", get_instance_host(GCP_INSTANCE_DB_US, GCP_ZONE_US))
  print("\n--- SERVICES ---")
  print(f"wrk2 in {GCP_ZONE_MANAGER} running @", get_instance_host(GCP_INSTANCE_APP_WRK2, GCP_ZONE_MANAGER))
  print(f"services in {GCP_ZONE_EU} running @", get_instance_host(GCP_INSTANCE_APP_EU, GCP_ZONE_EU))
  print(f"services in {GCP_ZONE_US} running @\n\n", get_instance_host(GCP_INSTANCE_APP_US, GCP_ZONE_US))
  
def gcp_start():
  from plumbum.cmd import ansible_playbook
  ansible_playbook["deploy/ansible/playbooks/start-datastores.yml", "-i", "deploy/tmp/ansible-inventory-gcp.cfg", "--extra-vars", "@deploy/tmp/ansible-vars.yml"] & FG
  display_progress_bar(30, "waiting for datastores to initialize")
  ansible_playbook["deploy/ansible/playbooks/start-app.yml", "-i", "deploy/tmp/ansible-inventory-gcp.cfg", "--extra-vars", "@deploy/tmp/ansible-vars.yml"] & FG

def gcp_stop():
  from plumbum.cmd import ansible_playbook
  ansible_playbook["deploy/ansible/playbooks/stop-datastores.yml", "-i", "deploy/tmp/ansible-inventory-gcp.cfg", "--extra-vars", "@deploy/tmp/ansible-vars.yml"] & FG
  ansible_playbook["deploy/ansible/playbooks/stop-app.yml", "-i", "deploy/tmp/ansible-inventory-gcp.cfg", "--extra-vars", "@deploy/tmp/ansible-vars.yml"] & FG

def gcp_restart():
  gcp_stop()
  gcp_start()
  
def gcp_clean():
  from plumbum.cmd import terraform
  import shutil

  terraform['-chdir=./deploy/terraform', 'destroy'] & FG
  if os.path.exists("deploy/tmp"):
    shutil.rmtree("deploy/tmp")
    print(f"[INFO] removed {BASE_DIR}/deploy/tmp/ directory")
 
def gcp_init_social_graph():
  print("[INFO] nothing to be done for gcp")
  exit(0)

def gcp_metrics(timestamp):
  metrics('gcp', timestamp)

def gcp_wrk2(threads, conns, duration, rate):
  from plumbum.cmd import ansible_playbook
  host_eu = get_instance_host(GCP_INSTANCE_APP_EU, GCP_ZONE_EU)
  timestamp = datetime.datetime.now().strftime("%Y-%m-%d_%H:%M:%S")
  run_workload(timestamp, 'gcp', f"http://{host_eu}:{APP_PORT}", threads, conns, duration, rate)
  gen_ansible_vars(timestamp, 'gcp')
  ansible_playbook["deploy/ansible/playbooks/gather-metrics.yml", "-i", "deploy/tmp/ansible-inventory-gcp.cfg", "--extra-vars", "@deploy/tmp/ansible-vars.yml"] & FG
  print(f"[INFO] metrics results saved at evaluation/gcp/{timestamp}/ in metrics-eu.yaml and metrics-us.yaml files")

# --------------------
# LOCAL
# --------------------

def local_init_social_graph():
  from plumbum import local
  with local.env(HOST_EU=f"http://127.0.0.1:{APP_PORT}", HOST_US=f"http://127.0.0.1:{APP_PORT}"):
    local['./scripts/init_social_graph.py'] & FG

def local_wrk2(threads, conns, duration, rate):
  timestamp = datetime.datetime.now().strftime("%Y-%m-%d_%H:%M:%S")
  run_workload(timestamp, 'local', f"http://127.0.0.1:{APP_PORT}", threads, conns, duration, rate)
  metrics('local', timestamp)

def local_metrics(timestamp):
  metrics('local', timestamp)

def local_storage_deploy():
  print("[INFO] nothing to be done for local")
  exit(0)

def local_storage_build():
  from plumbum.cmd import docker
  docker['build', '-t', 'mongodb-delayed:4.4.6', 'docker/mongodb-delayed/.'] & FG
  docker['build', '-t', 'mongodb-setup:4.4.6', 'docker/mongodb-setup/post-storage/.'] & FG
  docker['build', '-t', 'rabbitmq-setup:3.8', 'docker/rabbitmq-setup/write-home-timeline/.'] & FG

def local_storage_run():
  from plumbum.cmd import docker_compose
  docker_compose['up', '-d'] & FG
  display_progress_bar(30, "waiting for storages to be ready")

def local_storage_info():
  print("[INFO] nothing to be done for local")
  exit(0)

def local_storage_clean():
  from plumbum.cmd import docker_compose
  docker_compose['down'] & FG

if __name__ == "__main__":
  main_parser = argparse.ArgumentParser()
  command_parser = main_parser.add_subparsers(help='commands', dest='command')

  commands = [
    # gcp
    'configure', 'deploy', 'start', 'stop', 'info', 'restart', 'clean', 'info',
    # datastores
    'storage-build', 'storage-deploy', 'storage-run', 'storage-info', 'storage-clean',
    # eval
    'init-social-graph', 'wrk2', 'metrics',
  ]
  for cmd in commands:
    parser = command_parser.add_parser(cmd)
    parser.add_argument('--local', action='store_true', help="Running in localhost")
    parser.add_argument('--gcp', action='store_true',   help="Running in gcp")
    if cmd == 'wrk2':
      parser.add_argument('-t', '--threads', default=2, help="Number of threads")
      parser.add_argument('-c', '--conns', default=2, help="Number of connections")
      parser.add_argument('-d', '--duration', default=30, help="Duration")
      parser.add_argument('-r', '--rate', default=50, help="Number of requests per second")
    if cmd == 'metrics':
      parser.add_argument('-t', '--timestamp', help="Timestamp of workload")
      
  args = vars(main_parser.parse_args())
  command = args.pop('command').replace('-', '_')

  local = args.pop('local')
  gcp = args.pop('gcp')

  if local and gcp or not local and not gcp:
    print("[ERROR] one of --local or --gcp flgs needs to be provided")
    exit(-1)

  if local:
    command = 'local_' + command
  elif gcp:
    load_gcp_profile()
    command = 'gcp_' + command

  print(f"[INFO] ----- {command.upper().replace('_', ' ')} -----\n")
  getattr(sys.modules[__name__], command)(**args)
  print(f"[INFO] done!")
