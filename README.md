# Reece Iriye DevOps Portfolio

Here is my portfolio. I am deloying this on a Google Cloud Platform VM using Docker and on-prem on a 2-Node Raspberry Pi homelab using Kubernetes.

![Homepage](https://raw.githubusercontent.com/reece-iriye/portfolio-site/main/assets/root-readme/homepage.png)

## HTMX Portfolio Server

### Set-Up

Here are the following set-up's that need to take place when deploying my portfolio on-prem in a Kubernetes cluster and using Docker.

#### Single-Server Application Local Testing with Docker

First, set up a `.env` file using the variables from the `.env.example` file. All these values are required for successfully configuring an SMTP server using your choice of service. I tested these configurations using a Brevo SMTP server.

If you just want to test the application itself and not the rest of the components, the following commands:

```bash
cd htmx
docker buildx build --platform linux/amd64 -t portfolio-server --load .
docker run --env-file .env -p 8080:8080 portfolio-server
```

From here, you can fetch data from the HTMX/Go server using `http://localhost:8080`.

I also set up `air` for this Go application for local testing, so if you would prefer, install the `air` CLI tool and start up the server. This can also be accessed on `http://localhost:8080`.

```bash
cd htmx

go mod tidy && go mod download
go install github.com/air-verse/air@latest

air
```

Now, all code changes will be reflected in the development server immediately.


## Deploy Docker Compose Production Environment on GCP Virtual Machine


```bash
sudo adduser --disabled-password --gecos "" githubactions-deploy-user
sudo usermod -aG docker githubactions-deploy-user
```


### Create VM

Create a Google Cloud Platform VM on the [Google Cloud console](https://console.cloud.google.com). Specify all resource requirements when creating the VM. Because all containers for my portfolio application consume minimal resources, I opted for the smallest resource count to save money and resources, while being enough to run Docker containers and the `dockerd` daemon.

![Cloud Platform Home Page](https://raw.githubusercontent.com/reece-iriye/portfolio-site/main/assets/root-readme/gcloud-homepage-create-vm-button.png)

### Access VM Using SSH

```bash
gcloud init  # Follow log-in instructions
gcloud compute ssh --zone <ZONE> <INSTANCE_NAME> --project <PROJECT_NAME>
```

### Install Git on VM


```bash
sudo apt-get update
sudo apt-get install git
git --version
```

### Create GCP Service Account

Create a service account in your [Google Cloud Console](https://console.cloud.google.com/). Navigate to **IAM & Admin -> Service Accounts**. 

![Google Cloud Service Account](https://raw.githubusercontent.com/reece-iriye/portfolio-site/main/assets/root-readme/gcloud-service-account.png)

Click **Create Service Account** and name it `githubactions-deploy-user`. Assign the `IAP-Secured Tunnel User` role to the newly created user. Go to the **Keys** tab, then **Add Key -> Create new key -> JSON**. Download the JSON key file, which will be needed for Github Actions secrets. The reason why this needs to be done is to allow these GitHub Actions steps in the `deploy-gcp-vm` job to work:

```yaml
- name: Authenticate to Google Cloud
  uses: google-github-actions/auth@v1
  with:
    credentials_json: ${{ secrets.GCP_SA_KEY }}

- name: Set up GCP SDK
  uses: google-github-actions/setup-gcloud@v1

- name: Update Git Repository and Containers in VM over IAP
  run: |
    echo "SMTP_KEY=${{ secrets.SMTP_KEY }}" > .env.prod
    echo "SMTP_EMAIL=${{ secrets.SMTP_EMAIL }}" >> .env.prod
    echo "SMTP_HOST=${{ secrets.SMTP_HOST }}" >> .env.prod
    echo "SMTP_PORT=${{ secrets.SMTP_PORT }}" >> .env.prod
    echo "FROM_EMAIL=${{ secrets.FROM_EMAIL }}" >> .env.prod
    echo "TO_EMAIL=${{ secrets.TO_EMAIL }}" >> .env.prod

    echo "CLOUDFLARE_API_TOKEN=${{ secrets.CLOUDFLARE_API_TOKEN }}" >> .env.prod
    echo "ACME_ACCOUNT=${{ secrets.ACME_ACCOUNT }}" >> .env.prod

    gcloud compute scp .env.prod githubactions-deploy-user@${{ secrets.GCP_INSTANCE_NAME }}:~/portfolio-site/.env.prod \
      --tunnel-through-iap \
      --project=${{ secrets.GCP_PROJECT_ID }} \
      --zone=${{ secrets.GCP_ZONE }}

    gcloud compute ssh githubactions-deploy-user@${{ secrets.GCP_INSTANCE_NAME }} \
      --tunnel-through-iap \
      --project=${{ secrets.GCP_PROJECT_ID }} \
      --zone=${{ secrets.GCP_ZONE }} \
      --command="cd ~/portfolio-site && \
        git pull origin main && \
        docker compose --env-file .env.prod -f compose.prod.yaml pull && \
        docker compose --env-file .env.prod -f compose.prod.yaml build && \
        docker compose --env-file .env.prod -f compose.prod.yaml up -d"

    shred -u .env.prod
```

Now, the following secrets need to be added to the GitHub repository. Click on the **Settings** tab, then **Secrets and Variables -> Actions**, then configure the following secrets:
- `GCP_SA_KEY` -> the JSON file contents you downloaded
- `GCP_PROJECT_ID` -> the GCP Project ID (e.g. `devops-portfolio-123456`)
- `GCP_ZONE` -> the zone the VM is located in (e.g. `us-eastern1-a`)
- `GCP_INSTANCE_NAME` -> the name of the VM (e.g. `portfolio-vm`)

In order for this account to have access to the VM, you need to configure the account using your default GCP user that you used previously to SSH into the VM.

```bash
sudo adduser --disabled-password --gecos "" githubactions-deploy-user

sudo -u githubactions-deploy-user bash
cd ~
git clone https://github.com/reece-iriye/portfolio-site portfolio-site  # Or use forked repository if you forked it
cd ./portfolio-site
git branch --set-upstream-to=origin/main main

exit
```


### Add Docker to VM


I followed the instructions directly from the Docker docs for installing Docker onto a Debian Bookworm 12 VM, which can be found [here](https://docs.docker.com/engine/install/debian/#install-using-the-repository).

```bash
for pkg in docker.io docker-doc docker-compose podman-docker containerd runc; do sudo apt-get remove $pkg; done

# Add Docker's official GPG key:
sudo apt-get update
sudo apt-get install ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# Add the repository to Apt sources:
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update

# Install desired Docker plugins and containerd
sudo apt-get install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Test Docker Image Pull functionality
sudo docker run hello-world
```

To ensure that your user and `githubactions-deploy-user` can communicate with the Docker daeomon through this Unix socket `/var/run/docker.sock` owned by `root` with permissions `660`, you must run the following command:

```bash
sudo usermod -aG docker $USER
sudo usermod -aG docker githubactions-deploy-user
```

Then, log out and back in, and test `docker run hello-world` and `docker ps` on both accounts to test that it's working.


### Enviromnent Variable File Injection via GitHub Secrets

All environment variables that the application uses need to be configured. Reference all existing `.env.example` files, featured in both the `./grafana/` and `./htmx/` directories, and configure all of them in the same GitHub Actions secrets section as before.

![GitHub Secrets Page](https://raw.githubusercontent.com/reece-iriye/portfolio-site/main/assets/root-readme/github-secrets.png)

From here, continuous deployment onto the GCP VM via the `githubactions-deploy-user` service account is almost successfully configured. The `deploy-gcp-vm` job will be invoked upon pushing to main in this repository, or it can be run manually for repository owners. 


### SSL Configuration

This application uses Cloudflare Origin SSL certificates to ensure secure HTTPS connections between Cloudflare's edge servers and the Caddy reverse proxy. This approach eliminates SSL handshake issues that can occur with Let's Encrypt certificates when using Cloudflare's proxy service.

#### Generate Cloudflare Origin Certificate

1. Log into your [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. Navigate to **SSL/TLS → Origin Server**
3. Click **Create Certificate**
4. Choose **RSA (2048)** for broader compatibility
5. Set the certificate validity (up to 15 years)
6. Download both files:
   - **Certificate** → save as `cloudflare-origin.pem`
   - **Private Key** → save as `cloudflare-origin.key`

#### Certificate Deployment

The SSL certificates are managed through a secure directory structure in the repository inside the `caddy/certs/` directory (all files matching `*.key` pattern in any directory is in `.gitignore`):

```
nginx/certs/
├── cloudflare-origin.pem    # Public certificate (committed to repo)
└── cloudflare-origin.key    # Private key (gitignored, added manually)
```

**On your development machine:**
```bash
# Create the certs directory and add the public certificate
mkdir certs
cp ~/Downloads/cloudflare-origin.pem ./certs/cloudflare-origin.pem
git add certs/cloudflare-origin.pem certs/.gitignore
git commit -m "Add Cloudflare origin certificate"
```

**On the VM after deployment:**
```bash
# Log in as deployment user, navigate to the repository directory, copy private key
sudo su - githubactions-deploy-user
cd /path/to/repo/portfolio-site
cp ~/cloudflare-origin.key ./certs/cloudflare-origin-root-domain.key

# Set proper file permissions
chmod 600 ./certs/cloudflare-origin.key
chmod 644 ./certs/cloudflare-origin.pem

# Verify the private key is not tracked by git
git status  # Should not show the .key file
```

#### Cloudflare Dashboard Configuration

Ensure your Cloudflare SSL/TLS settings are configured correctly:

1. **SSL/TLS → Overview**: Set encryption mode to **"Full (strict)"**
2. **SSL/TLS → Edge Certificates**: 
   - Enable **"Always Use HTTPS"**
   - Set **Minimum TLS Version** to **1.2**
   - Enable **"TLS 1.3"** for better performance
3. Ensure your domain is **proxied** (orange cloud icon) in the DNS settings

#### GitHub Actions Integration

The private key must be manually added to the VM as it's not stored in version control for security reasons. The GitHub Actions workflow will automatically handle certificates by updating the `nginx` image created from `nginx/Containerfile` if `nginx/certs/cloudflare-origin.pem` changes on the GitHub repository or `nginx/certs/cloudflare-origin.key` is updated directly on the Virtual Machine. `nginx/conf.d/default.conf` maps to the certificate and private-key file inside of TLS settings.
