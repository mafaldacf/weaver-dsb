#!/usr/bin/env python3
import argparse
import googleapiclient.discovery
from google.oauth2 import service_account
import time
import sys
from plumbum import FG
import requests
from bs4 import BeautifulSoup
import re
import toml
import yaml
from time import sleep
from tqdm import tqdm

NUMBER_DOCKER_SWARM_SERVICE = 13
NUMBER_DOCKER_SWARM_NODES   = 3
# -----------
# GCP profile
# -----------
with open('gcp/config.yml', 'r') as file:
    config = yaml.safe_load(file)
    GCP_PROJECT_ID                = str(config['project_id'])
    GCP_USERNAME                  = str(config['username'])
    GCP_CLOUD_STORAGE_BUCKET_NAME = str(config['cloud_storage_bucket_name'])

# -----------------
# GCP configuration
# -----------------
# same as in terraform
APP_FOLDER_NAME           = "weaver-dsb-socialnetwork"
GCP_INSTANCE_NAME_MANAGER = "weaver-dsb-db-manager"
GCP_INSTANCE_NAME_EU      = "weaver-dsb-db-eu"
GCP_INSTANCE_NAME_US      = "weaver-dsb-db-us"
GCP_ZONE_MANAGER          = "europe-west3-a"
GCP_ZONE_EU               = "europe-west3-a"
GCP_ZONE_US               = "us-central1-a"

# --------------
# Dynamic Config
# ---------------
credentials = service_account.Credentials.from_service_account_file("gcp/credentials.json")
compute = googleapiclient.discovery.build('compute', 'v1', credentials=credentials)

# --------------------
# GCP
# --------------------

def get_instance_ips(instance_name, zone):
  instance = compute.instances().get(project=GCP_PROJECT_ID, zone=zone, instance=instance_name).execute()
  network_interface = instance['networkInterfaces'][0]
  # public, private
  return network_interface['accessConfigs'][0]['natIP'], network_interface['networkIP']

def storage_start():
  from plumbum.cmd import gcloud

  # get public ip for each instance
  public_ip_manager, _ = get_instance_ips(GCP_INSTANCE_NAME_MANAGER, GCP_ZONE_MANAGER)
  public_ip_eu, _ = get_instance_ips(GCP_INSTANCE_NAME_EU, GCP_ZONE_EU)
  public_ip_us, _ = get_instance_ips(GCP_INSTANCE_NAME_US, GCP_ZONE_US)

  # --- swarm manager
  cmd = f'sudo docker swarm init --advertise-addr {public_ip_manager}:2377'
  gcloud['compute', 'ssh', GCP_INSTANCE_NAME_MANAGER, '--zone', GCP_ZONE_MANAGER, '--command', cmd] & FG

  cmd = f'sudo docker network create --attachable -d overlay deathstarbench_network'
  gcloud['compute', 'ssh', GCP_INSTANCE_NAME_MANAGER, '--zone', GCP_ZONE_MANAGER, '--command', cmd] & FG

  cmd = f'sudo docker swarm join-token --quiet worker > token.txt'
  gcloud['compute', 'ssh', GCP_INSTANCE_NAME_MANAGER, '--zone', GCP_ZONE_MANAGER, '--command', cmd] & FG

  gcloud['compute', 'scp', f"{GCP_INSTANCE_NAME_MANAGER}:token.txt", 'gcp/token.txt'] & FG

  f = open('gcp/token.txt', 'r')
  token = f.read().strip()
  f.close()

  # --- nodes
  cmd = f'sudo docker swarm join --token {token} {public_ip_manager}:2377 --advertise-addr {public_ip_eu}'
  gcloud['compute', 'ssh', GCP_INSTANCE_NAME_EU, '--zone', GCP_ZONE_EU, '--command', cmd] & FG

  cmd = f'sudo docker swarm join --token {token} {public_ip_manager}:2377 --advertise-addr {public_ip_us}'
  gcloud['compute', 'ssh', GCP_INSTANCE_NAME_US, '--zone', GCP_ZONE_US ,'--command', cmd] & FG

  # --- manager
  cmd = f'sudo docker stack deploy --with-registry-auth --compose-file ~/{APP_FOLDER_NAME}/docker-compose.yml socialnetwork'
  gcloud['compute', 'ssh', GCP_INSTANCE_NAME_MANAGER, '--command', cmd] & FG

  print("[INFO] waiting 30 seconds for docker swarm...")
  for _ in tqdm(range(30)):
      sleep(1)

  try:
    cmd_nodes_ready_counter = "sudo docker node ls --format '{{.Hostname}}: {{.Status}}' | grep 'Ready' | wc -l"
    cmd = f"if [ $({cmd_nodes_ready_counter}) -eq {NUMBER_DOCKER_SWARM_NODES} ]; then echo docker swarm nodes OK!; else exit 1; fi"
    gcloud['compute', 'ssh', GCP_INSTANCE_NAME_MANAGER, '--command', cmd] & FG
  except Exception as e:
    print(f"[ERROR] not all nodes are ready\n\n{e}")
    exit(-1)

  try:
    cmd_services_counter = "sudo docker stack services socialnetwork --format '{{.Name}}: {{.Replicas}}' | grep '1/1' | wc -l"
    cmd = f"if [ $({cmd_services_counter}) -eq {NUMBER_DOCKER_SWARM_SERVICE} ]; then echo docker swarm services OK!; else exit 1; fi"
    gcloud['compute', 'ssh', GCP_INSTANCE_NAME_MANAGER, '--command', cmd] & FG
  except Exception as e:
    print(f"[ERROR] not all services are replicated\n\n{e}")
    exit(-1)

def storage_info():
  from plumbum.cmd import gcloud
  cmd = f'sudo docker node ls'
  gcloud['compute', 'ssh', GCP_INSTANCE_NAME_MANAGER, '--command', cmd] & FG
  cmd = f'sudo docker service ls'
  gcloud['compute', 'ssh', GCP_INSTANCE_NAME_MANAGER, '--command', cmd] & FG

  print()
  public_ip_manager, _ = get_instance_ips(GCP_INSTANCE_NAME_MANAGER, GCP_ZONE_MANAGER)
  print("storage manager running @", public_ip_manager)
  public_ip_eu, _ = get_instance_ips(GCP_INSTANCE_NAME_EU, GCP_ZONE_EU)
  print(f"storage in {GCP_ZONE_EU} running @", public_ip_eu)
  public_ip_us, _ = get_instance_ips(GCP_INSTANCE_NAME_US, GCP_ZONE_US)
  print(f"storage in {GCP_ZONE_US} running @", public_ip_us)
  print()

  gen_weaver_config(public_ip_eu, public_ip_us)

def gen_weaver_config(public_ip_eu = "0.0.0.0", public_ip_us = "0.0.0.0"):
  data = toml.load("weaver.toml")

  for _, config in data.items():
    for entry, values in config.items():
      if entry in ['mongodb_address', 'redis_address', 'rabbitmq_address', 'memcached_address']:
        values["local"] = public_ip_eu
        values["europe-west3"] = public_ip_eu
        values["us-central-1"] = public_ip_us

  f = open("weaver-gcp.toml",'w')
  toml.dump(data, f)
  f.close()
  print("[INFO] generated app config at weaver-gcp.toml")

def storage_deploy():
  from plumbum.cmd import terraform, mkdir, cp, gsutil, rm
  from plumbum.cmd import gcloud

  mkdir['-p', f'tmp/{APP_FOLDER_NAME}'] & FG
  cp['-r', 'docker-compose.yml', 'docker', 'config', APP_FOLDER_NAME] & FG
  gsutil['cp', '-r', f'tmp/{APP_FOLDER_NAME}', f'gs://{GCP_CLOUD_STORAGE_BUCKET_NAME}/'] & FG
  rm['-r', 'tmp'] & FG

  terraform['-chdir=./terraform', 'init'] & FG
  terraform['-chdir=./terraform', 'apply'] & FG

  print("[INFO] waiting 200 seconds to install all dependencies in GCP instances...")
  for _ in tqdm(range(200)):
      sleep(1)

  def move_app_folder():
    return f'sudo mv /{APP_FOLDER_NAME} /home/{GCP_USERNAME}/{APP_FOLDER_NAME} 2>/dev/null; true'
  gcloud['compute', 'ssh', GCP_INSTANCE_NAME_MANAGER, '--zone', GCP_ZONE_MANAGER, '--command', move_app_folder()] & FG
  gcloud['compute', 'ssh', GCP_INSTANCE_NAME_EU, '--zone', GCP_ZONE_EU, '--command', move_app_folder()] & FG
  gcloud['compute', 'ssh', GCP_INSTANCE_NAME_US, '--zone', GCP_ZONE_US, '--command', move_app_folder()] & FG
  
  def validate_app_folder(instance_identifier):
    return f'if [ -d "/home/{GCP_USERNAME}/{APP_FOLDER_NAME}" ]; then echo "{instance_identifier}: app folder OK!"; fi'
  try:
    gcloud['compute', 'ssh', GCP_INSTANCE_NAME_MANAGER, '--zone', GCP_ZONE_MANAGER, '--command', validate_app_folder('manager')] & FG
    gcloud['compute', 'ssh', GCP_INSTANCE_NAME_EU, '--zone', GCP_ZONE_EU, '--command', validate_app_folder('storage @ eu')] & FG
    gcloud['compute', 'ssh', GCP_INSTANCE_NAME_US, '--zone', GCP_ZONE_US, '--command', validate_app_folder('storage @ us')] & FG
  except Exception as e:
    print(f"[ERROR] app folder missing in gcp instance:\n\n{e}")
    exit(-1)

  def validate_docker_images(instance_identifier):
    return f'if [ $(sudo docker images | tail -n +2 | wc -l) -eq 6 ]; then echo "{instance_identifier}: docker images OK!"; else exit 1; fi'
  try:
    gcloud['compute', 'ssh', GCP_INSTANCE_NAME_MANAGER, '--zone', GCP_ZONE_MANAGER, '--command', validate_docker_images('manager')] & FG
    gcloud['compute', 'ssh', GCP_INSTANCE_NAME_EU, '--zone', GCP_ZONE_EU, '--command', validate_docker_images('storage @ eu')] & FG
    gcloud['compute', 'ssh', GCP_INSTANCE_NAME_US, '--zone', GCP_ZONE_US, '--command', validate_docker_images('storage @ us')] & FG
  except Exception as e:
    print(f"[ERROR] app folder missing in gcp instance:\n\n{e}")
    exit(-1)
  
  
def storage_clean():
  from plumbum.cmd import terraform
  terraform['-chdir=./terraform', 'destroy'] & FG

def init_social_graph(address):
  #TODO
  pass

def wrk2():
  #TODO
  pass

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

def metrics(deployment_type='gke'):
  from plumbum.cmd import weaver
  import re

  pattern = re.compile(r'^.*│.*│.*│.*│\s*(\d+\.?\d*)\s*│.*$', re.MULTILINE)

  # wkr2 api
  compose_post_duration_metrics = weaver[deployment_type, 'metrics', 'sn_compose_post_duration_ms']()
  compose_post_duration_avg_ms = sum(float(value) for value in pattern.findall(compose_post_duration_metrics))
  # compose post service
  composed_posts_metrics = weaver[deployment_type, 'metrics', 'sn_composed_posts']()
  composed_posts_count = sum(int(value) for value in pattern.findall(composed_posts_metrics))
  # post storage service
  write_post_duration_metrics = weaver[deployment_type, 'metrics', 'sn_write_post_duration_ms']()
  write_post_duration_avg_ms = sum(float(value) for value in pattern.findall(write_post_duration_metrics))
  # write home timeline service
  queue_duration_metrics = weaver[deployment_type, 'metrics', 'sn_queue_duration_ms']()
  queue_duration_avg_ms = sum(float(value) for value in pattern.findall(queue_duration_metrics))
  received_notifications_metrics = weaver[deployment_type, 'metrics', 'sn_received_notifications']()
  received_notifications_count = sum(int(value) for value in pattern.findall(received_notifications_metrics))
  inconsitencies_metrics = weaver[deployment_type, 'metrics', 'sn_inconsistencies']()
  inconsistencies_count = sum(int(value) for value in pattern.findall(inconsitencies_metrics))

  pc_inconsistencies = "{:.2f}".format((inconsistencies_count / composed_posts_count) * 100)
  pc_received_notifications = "{:.2f}".format((received_notifications_count / composed_posts_count) * 100)
  compose_post_duration_avg_ms = "{:.4f}".format(compose_post_duration_avg_ms)
  write_post_duration_avg_ms = "{:.4f}".format(write_post_duration_avg_ms)
  queue_duration_avg_ms = "{:.4f}".format(queue_duration_avg_ms)

  print(f"# composed posts:\t\t{composed_posts_count}")
  print(f"# received notifications:\t{composed_posts_count} ({pc_received_notifications}%)")
  print(f"# inconsistencies:\t\t{inconsistencies_count}")
  print(f"% inconsistencies:\t\t{pc_inconsistencies}%")
  print(f"% avg. compose post duration:\t{compose_post_duration_avg_ms}ms")
  print(f"% avg. write post duration:\t{write_post_duration_avg_ms}ms")
  print(f"% avg. queue duration:\t\t{queue_duration_avg_ms}ms")

# --------------------
# LOCAL
# --------------------

def local_init_social_graph():
  from plumbum import local
  with local.env(HOST_EU="http://localhost:9000", HOST_US="http://localhost:9000"):
    local['./scripts/init_social_graph.py'] & FG

#./manager.py wrk2 --local -t 2 -c 4 -d 5 -r 50
def local_wrk2(threads=4, conns=2, duration=5, reqs=50):
  from plumbum import local
  with local.env(HOST_EU="http://localhost:8000", HOST_US="http://localhost:8000"):
      wrk2 = local['./wrk2/wrk']
      wrk2['-D', 'exp', '-t', str(threads), '-c', str(conns), '-d', str(duration), '-L', '-s', './wrk2/scripts/social-network/compose-post.lua', 'http://localhost:8000/wrk2-api/post/compose', '-R', str(reqs)] & FG

def local_metrics():
  metrics('multi')

def local_storage_deploy():
  from plumbum.cmd import docker
  docker['build', '-t', 'mongodb-delayed:4.4.6', 'docker/mongodb-delayed/.'] & FG
  docker['build', '-t', 'mongodb-setup:4.4.6', 'docker/mongodb-setup/post-storage/.'] & FG
  docker['build', '-t', 'rabbitmq-setup:3.8', 'docker/rabbitmq-setup/write-home-timeline/.'] & FG

def local_storage_start():
  from plumbum.cmd import docker_compose
  docker_compose['up', '-d'] & FG

def local_storage_clean():
  from plumbum.cmd import docker_compose
  docker_compose['down'] & FG

if __name__ == "__main__":
  main_parser = argparse.ArgumentParser()
  command_parser = main_parser.add_subparsers(help='commands', dest='command')

  commands = [
    # ----------------
    # gcp datastores
    'storage-deploy', 'storage-start', 'storage-info', 'storage-clean', 
    # gcp app
    'init-social-graph', 'wrk2', 'metrics',
  ]
  for cmd in commands:
    parser = command_parser.add_parser(cmd)
    parser.add_argument('--local', action='store_true', help="Running in localhost")
    if cmd == 'wrk2':
      parser.add_argument('-t', '--threads', default=1, help="Number of threads")
      parser.add_argument('-c', '--conns', default=1, help="Number of connections")
      parser.add_argument('-d', '--duration', default=1, help="Duration")
      parser.add_argument('-r', '--reqs', default=1, help="Number of requests per second")

  args = vars(main_parser.parse_args())
  command = args.pop('command').replace('-', '_')
  local = args.pop('local')

  if local:
    command = 'local_' + command

  print(f"[INFO] ----- {command.upper()} -----")
  getattr(sys.modules[__name__], command)(**args)
  print(f"[INFO] done!")