#!/usr/bin/env python3
import argparse
import googleapiclient.discovery
from google.oauth2 import service_account
import sys
from plumbum import FG
import toml
import yaml
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
GCP_BUCKET   = None
GCP_CREDENTIALS                 = None
GCP_COMPUTE                     = None

# ---------------------
# GCP app configuration
# ---------------------
# same as in terraform
APP_FOLDER_NAME           = "socialnetwork"
GCP_INSTANCE_APP_MANAGER  = "weaver-dsb-app-manager"
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
  global GCP_PROJECT_ID, GCP_USERNAME, GCP_BUCKET, GCP_COMPUTE
  try:
    with open('gcp/config.yml', 'r') as file:
      config = yaml.safe_load(file)
      GCP_PROJECT_ID  = str(config['project_id'])
      GCP_USERNAME    = str(config['username'])
      GCP_BUCKET      = str(config['bucket_name'])
    GCP_CREDENTIALS   = service_account.Credentials.from_service_account_file("gcp/credentials.json")
    GCP_COMPUTE = googleapiclient.discovery.build('compute', 'v1', credentials=GCP_CREDENTIALS)
  except Exception as e:
      print(f"[ERROR] error loading gcp profile: {e}")
      exit(-1)

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

  # display progress bar
  def tqdm_progress(duration):
      print(f"[INFO] running workload for {duration} seconds...")
      for _ in tqdm(range(int(duration))):
          sleep(1)

  progress_thread = threading.Thread(target=tqdm_progress, args=(duration,))
  progress_thread.start()

  from plumbum import local
  with local.env(HOST_EU=url, HOST_US=url):
    wrk2 = local['./wrk2/wrk']
    output = wrk2['-D', 'exp', '-t', str(threads), '-c', str(conns), '-d', str(duration), '-L', '-s', './wrk2/scripts/social-network/compose-post.lua', f'{url}/wrk2-api/post/compose', '-R', str(rate)]()
  
    filepath = f"evaluation/{deployment}/{timestamp}.workload"
    with open(filepath, "w") as f:
      f.write(output)

    print(output)
    print(f"[INFO] workload results saved at {filepath}")

  progress_thread.join()
  return output

def gen_weaver_config_gcp(public_ip_db_eu, public_ip_db_us):
  data = toml.load("weaver-local-eu.toml")

  # europe
  for _, config in data.items():
    if 'mongodb_address' in config:
      config['mongodb_address'] = public_ip_db_eu
    if 'redis_address' in config:
      config['redis_address'] = public_ip_db_eu
    if 'rabbitmq_address' in config:
      config['rabbitmq_address'] = public_ip_db_eu
    if 'memcached_address' in config:
      config['memcached_address'] = public_ip_db_eu
  f = open("weaver-gcp-eu.toml",'w')
  toml.dump(data, f)
  f.close()

  # us
  data = toml.load("weaver-local-us.toml")
  for _, config in data.items():
    if 'mongodb_address' in config:
      config['mongodb_address'] = public_ip_db_us
    if 'redis_address' in config:
      config['redis_address'] = public_ip_db_us
    if 'rabbitmq_address' in config:
      config['rabbitmq_address'] = public_ip_db_us
    if 'memcached_address' in config:
      config['memcached_address'] = public_ip_db_us
  f = open("weaver-gcp-us.toml",'w')
  toml.dump(data, f)
  f.close()

  print("[INFO] generated app config for GCP at 'weaver-gcp-eu.toml' and 'weaver-gcp-us.toml'")

def gen_ansible_inventory_gcp():
  from jinja2 import Environment
  import textwrap

  # datastores
  host_db_manager = get_instance_host(GCP_INSTANCE_DB_MANAGER, GCP_ZONE_MANAGER)
  host_db_eu = get_instance_host(GCP_INSTANCE_DB_EU, GCP_ZONE_EU)
  host_db_us = get_instance_host(GCP_INSTANCE_DB_US, GCP_ZONE_US)
  # app
  #host_app_manager = get_instance_host(GCP_INSTANCE_APP_EU, GCP_ZONE_MANAGER)
  host_app_eu = get_instance_host(GCP_INSTANCE_APP_EU, GCP_ZONE_EU)
  host_app_us = get_instance_host(GCP_INSTANCE_APP_US, GCP_ZONE_US)

  template = """
    [swarm_manager]
    weaver-dsb-db-manager   ansible_host={{ host_db_manager }} zone="europe-west3-a" user=mafaldacf

    [swarm_workers]
    weaver-dsb-db-eu        ansible_host={{ host_db_eu }} zone="europe-west3-a" user=mafaldacf
    weaver-dsb-db-us        ansible_host={{ host_db_us }} zone="us-central1-a"  user=mafaldacf

    [app_manager]
    weaver-dsb-app-manager  ansible_host={{ host_app_manager }} zone="europe-west3-a" user=mafaldacf

    [app_services]
    weaver-dsb-app-eu       ansible_host={{ host_app_eu }}  region="eu" zone="europe-west3-a" user=mafaldacf app_port="9000"
    weaver-dsb-app-us       ansible_host={{ host_app_us }}   region="us" zone="us-central1-a"  user=mafaldacf app_port="9001"
  """
  inventory = Environment().from_string(template).render({
    'host_db_manager': host_db_manager,
    'host_db_eu': host_db_eu,
    'host_db_us': host_db_us,
    'host_app_manager': '127.0.0.1',
    'host_app_eu': host_app_eu,
    'host_app_us': host_app_us,
  })

  filename = "ansible/inventory-gcp.cfg"
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

def metrics(deployment_type='gke', timestamp=None, local=True):
  from plumbum.cmd import weaver, grep
  import re

  primary_region = 'europe-west3'
  secondary_region = 'us-central-1' if not local else primary_region

  pattern = re.compile(r'^.*│.*│.*│.*│\s*(\d+\.?\d*)\s*│.*$', re.MULTILINE)

  def get_filter_metrics(deployment_type, metric_name, region):
    #return (weaver[deployment_type, 'metrics', metric_name] | grep[region])()
    return weaver[deployment_type, 'metrics', metric_name]()

  # wkr2 api
  compose_post_duration_metrics = get_filter_metrics(deployment_type, 'sn_compose_post_duration_ms', primary_region)
  compose_post_duration_metrics_values = pattern.findall(compose_post_duration_metrics)
  compose_post_duration_avg_ms = sum(float(value) for value in compose_post_duration_metrics_values)/len(compose_post_duration_metrics_values)
  # compose post service
  composed_posts_metrics = get_filter_metrics(deployment_type, 'sn_composed_posts', primary_region)
  composed_posts_count = sum(int(value) for value in pattern.findall(composed_posts_metrics))
  # post storage service
  write_post_duration_metrics = get_filter_metrics(deployment_type, 'sn_write_post_duration_ms', primary_region)
  write_post_duration_metrics_values = pattern.findall(write_post_duration_metrics)
  write_post_duration_avg_ms = sum(float(value) for value in write_post_duration_metrics_values)/len(write_post_duration_metrics_values)
  # write home timeline service
  queue_duration_metrics = get_filter_metrics(deployment_type, 'sn_queue_duration_ms', secondary_region)
  queue_duration_metrics_values = pattern.findall(queue_duration_metrics)
  queue_duration_avg_ms = sum(float(value) for value in queue_duration_metrics_values)/len(queue_duration_metrics_values)
  received_notifications_metrics = get_filter_metrics(deployment_type, 'sn_received_notifications', secondary_region)
  received_notifications_count = sum(int(value) for value in pattern.findall(received_notifications_metrics))
  inconsitencies_metrics = get_filter_metrics(deployment_type, 'sn_inconsistencies', secondary_region)
  inconsitencies_metrics = weaver[deployment_type, 'metrics', 'sn_inconsistencies']()
  inconsistencies_count = sum(int(value) for value in pattern.findall(inconsitencies_metrics))

  pc_inconsistencies = "{:.2f}".format((inconsistencies_count / composed_posts_count) * 100)
  pc_received_notifications = "{:.2f}".format((received_notifications_count / composed_posts_count) * 100)
  compose_post_duration_avg_ms = "{:.2f}".format(compose_post_duration_avg_ms)
  write_post_duration_avg_ms = "{:.2f}".format(write_post_duration_avg_ms)
  queue_duration_avg_ms = "{:.2f}".format(queue_duration_avg_ms)

  results = f"""
    # composed posts:\t\t\t{composed_posts_count}
    # received notifications @ US:\t{received_notifications_count} ({pc_received_notifications}%)
    # inconsistencies @ US:\t\t{inconsistencies_count}
    % inconsistencies @ US:\t\t{pc_inconsistencies}%
    > avg. compose post duration:\t{compose_post_duration_avg_ms}ms
    > avg. write post duration:\t\t{write_post_duration_avg_ms}ms
    > avg. queue duration @ US:\t\t{queue_duration_avg_ms}ms
  """
  print(results)

  # save file if we ran workload
  if timestamp:
    eval_folder = 'local' if deployment_type == 'multi' else 'gke'
    filepath = f"evaluation/{eval_folder}/{timestamp}.metrics"
    with open(filepath, "w") as f:
      f.write(results)
    print(f"[INFO] evaluation results saved at {filepath}")

# --------------------
# GCP
# --------------------

def gcp_configure(bucket):
  from plumbum.cmd import gcloud

  try:
    # create bucket
    print(f"[INFO] (1/3) creating bucket {bucket}")
    gcloud['storage', '--project', GCP_PROJECT_ID, 'buckets', 'create', f'gs://{bucket}', '--public-access-prevention'] & FG
  except Exception as e:
    print(f"[ERROR] could not create bucket: {e}\n\n")

  try:
    print("[INFO] (2/3) configuring firewalls")
    # configure firewalls
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

  try:
    print("[INFO] (3/3) creating artifact registry for docker images")
  
  except Exception as e:
    print(f"[ERROR] could not create artifacts repository for docker images: {e}\n\n")

def gcp_build():
  from plumbum.cmd import rm, go, mkdir, cp, gsutil, rm

  # upload files to gcp cloud storage
  mkdir['-p', f'tmp/{APP_FOLDER_NAME}'] & FG
  cp['-r', 'docker-compose.yml', 'docker', 'requirements.txt', f'tmp/{APP_FOLDER_NAME}'] & FG
  gsutil['cp', '-r', f'tmp/{APP_FOLDER_NAME}', f'gs://{GCP_BUCKET}/'] & FG
  rm['-r', 'tmp'] & FG

def gcp_deploy():
  from plumbum.cmd import terraform
  from plumbum.cmd import gcloud

  terraform['-chdir=./terraform', 'init'] & FG
  terraform['-chdir=./terraform', 'apply'] & FG

  waiting_time = 300
  while waiting_time > 10:
    print(f"[INFO] waiting {waiting_time} seconds to install all dependencies in GCP instances...")
    for _ in tqdm(range(waiting_time)):
      sleep(1)
    # check if dependencies file was created, otherwise we go for a second round of waiting
    try:
      cmd = f'if [ -f "/deps.ready" ]; then echo "{GCP_INSTANCE_DB_MANAGER}: dependencies OK!"; else exit 1; fi'
      gcloud['compute', 'ssh', GCP_INSTANCE_DB_MANAGER, '--zone', GCP_ZONE_MANAGER, '--command', cmd] & FG
      #cmd = f'if [ -f "/deps.ready" ]; then echo "{GCP_INSTANCE_APP_MANAGER}: dependencies OK!"; else exit 1; fi'
      #gcloud['compute', 'ssh', GCP_INSTANCE_APP_MANAGER, '--zone', GCP_ZONE_MANAGER, '--command', cmd] & FG
      break
    except Exception as e:
      waiting_time = int(waiting_time - 50)
      pass


  # move folder from root dir to user dir
  def move_app_folder():
    return f'sudo mv /{APP_FOLDER_NAME} /home/{GCP_USERNAME}/{APP_FOLDER_NAME} 2>/dev/null; true'
  gcloud['compute', 'ssh', GCP_INSTANCE_DB_MANAGER, '--zone', GCP_ZONE_MANAGER, '--command', move_app_folder()] & FG
  gcloud['compute', 'ssh', GCP_INSTANCE_DB_EU, '--zone', GCP_ZONE_EU, '--command', move_app_folder()] & FG
  gcloud['compute', 'ssh', GCP_INSTANCE_DB_US, '--zone', GCP_ZONE_US, '--command', move_app_folder()] & FG
  
  def validate_app_folder(instance_identifier):
    return f'if [ -d "/home/{GCP_USERNAME}/{APP_FOLDER_NAME}" ]; then echo "{instance_identifier}: app folder OK!"; else exit 1; fi'
  try:
    gcloud['compute', 'ssh', GCP_INSTANCE_DB_MANAGER, '--zone', GCP_ZONE_MANAGER, '--command', validate_app_folder('manager')] & FG
    gcloud['compute', 'ssh', GCP_INSTANCE_DB_EU, '--zone', GCP_ZONE_EU, '--command', validate_app_folder('storage @ eu')] & FG
    gcloud['compute', 'ssh', GCP_INSTANCE_DB_US, '--zone', GCP_ZONE_US, '--command', validate_app_folder('storage @ us')] & FG
  except Exception as e:
    print(f"[ERROR] app folder missing in gcp instance:\n\n{e}")
    exit(-1)

  def validate_docker_images(instance_identifier):
    return f'if [ $(sudo docker images | tail -n +2 | wc -l) -eq 6 ]; then echo "{instance_identifier}: docker images OK!"; else exit 1; fi'
  try:
    gcloud['compute', 'ssh', GCP_INSTANCE_DB_MANAGER, '--zone', GCP_ZONE_MANAGER, '--command', validate_docker_images('manager')] & FG
    gcloud['compute', 'ssh', GCP_INSTANCE_DB_EU, '--zone', GCP_ZONE_EU, '--command', validate_docker_images('storage @ eu')] & FG
    gcloud['compute', 'ssh', GCP_INSTANCE_DB_US, '--zone', GCP_ZONE_US, '--command', validate_docker_images('storage @ us')] & FG
  except Exception as e:
    print(f"[ERROR] docker images missing:\n\n{e}")
    exit(-1)
 
  public_ip_db_eu = get_instance_host(GCP_INSTANCE_DB_EU, GCP_ZONE_EU)
  public_ip_db_us = get_instance_host(GCP_INSTANCE_DB_US, GCP_ZONE_US)

  gen_weaver_config_gcp(public_ip_db_eu, public_ip_db_us)

  # copy app binary and weaver config for app instances @ eu & us
  gcloud['compute', 'scp', '--recurse', '--zone', GCP_ZONE_EU, 'pkg', 'main.go', 'go.mod', 'go.sum', 'weaver-gcp-eu.toml', f'{GCP_USERNAME}@{GCP_INSTANCE_APP_EU}:'] & FG
  gcloud['compute', 'scp', '--recurse', '--zone', GCP_ZONE_US, 'pkg', 'main.go', 'go.mod', 'go.sum', 'weaver-gcp-us.toml', f'{GCP_USERNAME}@{GCP_INSTANCE_APP_US}:'] & FG

  # copy manager.py script and python requirements for app manager, and install requirements
  #gcloud['compute', 'scp', '--zone', GCP_ZONE_MANAGER, 'manager.py', 'requirements.txt', f'{GCP_USERNAME}@{GCP_INSTANCE_APP_EU}:'] & FG
  #cmd = 'pip install -r requirements.txt'
  #gcloud['compute', 'ssh', GCP_INSTANCE_APP_MANAGER, '--zone', GCP_ZONE_MANAGER, '--command', cmd] & FG

  gen_ansible_inventory_gcp()
    

def gcp_info():
  from plumbum.cmd import gcloud
  cmd = f'sudo docker node ls'
  gcloud['compute', 'ssh', GCP_INSTANCE_DB_MANAGER, '--command', cmd] & FG
  cmd = f'sudo docker service ls'
  gcloud['compute', 'ssh', GCP_INSTANCE_DB_MANAGER, '--command', cmd] & FG

  print()
  print("--- DATASTORES ---")
  public_ip_manager = get_instance_host(GCP_INSTANCE_DB_MANAGER, GCP_ZONE_MANAGER)
  print("storage manager running @", public_ip_manager)
  public_ip_eu = get_instance_host(GCP_INSTANCE_DB_EU, GCP_ZONE_EU)
  print(f"storage in {GCP_ZONE_EU} running @", public_ip_eu)
  public_ip_us = get_instance_host(GCP_INSTANCE_DB_US, GCP_ZONE_US)
  print(f"storage in {GCP_ZONE_US} running @", public_ip_us)
  print()

  print("--- SERVICES ---")
  #public_ip_manager = get_instance_host(GCP_INSTANCE_APP_MANAGER, GCP_ZONE_MANAGER)
  #print("app manager running @", public_ip_manager)
  public_ip_eu = get_instance_host(GCP_INSTANCE_APP_EU, GCP_ZONE_EU)
  print(f"services in {GCP_ZONE_EU} running @", public_ip_eu)
  public_ip_us = get_instance_host(GCP_INSTANCE_APP_US, GCP_ZONE_US)
  print(f"services in {GCP_ZONE_US} running @", public_ip_us)
  print()
  print()
  
def gcp_run():
  from plumbum.cmd import ansible_playbook
  gen_ansible_inventory_gcp()

  ansible_playbook["ansible/start-datastores.yml", "-i", "ansible/inventory-gcp.cfg"] & FG
  print("[INFO] waiting 60 seconds for datastores to initialize...")
  for _ in tqdm(range(60)):
      sleep(1)

  ansible_playbook["ansible/start-app.yml", "-i", "ansible/inventory-gcp.cfg"] & FG

def gcp_stop():
  from plumbum.cmd import ansible_playbook
  ansible_playbook["ansible/stop-datastores.yml", "-i", "ansible/inventory-gcp.cfg"] & FG
  ansible_playbook["ansible/stop-app.yml", "-i", "ansible/inventory-gcp.cfg"] & FG

def gcp_restart():
  gcp_stop()
  gcp_run()

  
def gcp_clean():
  from plumbum.cmd import terraform
  terraform['-chdir=./terraform', 'destroy'] & FG

def gcp_init_social_graph():
  pass

def gcp_metrics():
  metrics('multi', None)

def gcp_wrk2(threads, conns, duration, rate):
  host_eu = get_instance_host(GCP_INSTANCE_APP_EU, GCP_ZONE_EU)
  timestamp = datetime.datetime.now().strftime("%Y-%m-%d_%H:%M:%S")
  run_workload(timestamp, 'gcp', f"http://{host_eu}:{APP_PORT}", threads, conns, duration, rate)
  #FIXME
  #metrics('gcp', timestamp)


# --------------------
# LOCAL
# --------------------

def local_init_social_graph(address):
  from plumbum import local
  with local.env(HOST_EU=f"http://127.0.0.1:{APP_PORT}", HOST_US=f"http://127.0.0.1:{APP_PORT}"):
    local['./scripts/init_social_graph.py'] & FG

def local_wrk2(threads, conns, duration, rate):
  timestamp = datetime.datetime.now().strftime("%Y-%m-%d_%H:%M:%S")
  run_workload(timestamp, 'local', f"http://127.0.0.1:{APP_PORT}", threads, conns, duration, rate)
  metrics('multi', timestamp, True)

def local_metrics():
  metrics('multi', None, True)

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
  print("[INFO] waiting 30 seconds for storages to be ready...")
  for _ in tqdm(range(30)):
      sleep(1)

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
    'configure', 'build', 'deploy', 'run', 'stop', 'info', 'restart', 'clean', 'info',
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
    if cmd == 'configure':
      parser.add_argument('-b', '--bucket', help="Name of the bucket")
      
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
