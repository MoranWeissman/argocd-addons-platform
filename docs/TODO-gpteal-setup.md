## GPTeal External Access Setup — TODO

### Context
AAP runs on EKS outside the internal network. To use GPTeal from the cluster, we need mTLS authentication via the external endpoint (`sapi-test.merck.com`).

For local development, use a personal API key with a direct provider (OpenAI/Claude/etc.) until this setup is complete.

### Steps

- [ ] **1. Request NPA (Non-Person Account)**
  - Follow: [Request NPA Guide](https://collaboration.merck.com) (internal Confluence)
  - The NPA's CN (Common Name) will be tied to the client certificate

- [ ] **2. Generate SSL Client Certificate**
  - Follow: [Client Certificate Guide](https://collaboration.merck.com) (internal Confluence)
  - Extract `.crt`, `.key`, and `.pfx` files
  - Store securely — these will be mounted as a K8s Secret

- [ ] **3. Link API Key to Client Certificate**
  - Create Jira ticket on [WAPS board](https://issues.merck.com/projects/WAPS/summary)
  - Issue Type: **Service Request**
  - Summary: **Contact Support - GPT API**
  - Environment: **Test** (start with test, then repeat for prod)
  - Description:
    ```
    Please update the certificate used for mTLS traffic for the GPT API
    Certificate name: <your certificate name>
    API key name: <your API key name>
    ```

- [ ] **4. Deploy cert to EKS**
  - Create K8s Secret with `.crt` and `.key` files
  - Mount into AAP pod
  - Configure AAP to use cert for GPTeal HTTP client

### Endpoints
| Environment | URL | Auth |
|---|---|---|
| Test (external) | `https://sapi-test.merck.com/gpt` | mTLS + `X-Merck-APIKey` |
| Prod (external) | `https://sapi.merck.com/gpt` | mTLS + `X-Merck-APIKey` |
| Test (internal) | `https://iapi-test.merck.com/gpt` | `X-Merck-APIKey` only |
| Prod (internal) | `https://iapi.merck.com/gpt` | `X-Merck-APIKey` only |
