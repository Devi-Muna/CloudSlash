/**
 * CloudSlash License Server (Freemius Proxy)
 * Securely communicates with Freemius API to verify licenses.
 */

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    // Health Check
    if (request.method === "GET" && url.pathname === "/") {
      return new Response("CloudSlash License Server Online", { status: 200 });
    }

    // Verify Endpoint
    if (request.method === "POST" && url.pathname === "/verify") {
      try {
        const { licenseKey } = await request.json();
        if (!licenseKey)
          return new Response("Missing licenseKey", { status: 400 });

        // validate env vars
        if (
          !env.FREEMIUS_SECRET_KEY ||
          !env.PRODUCT_ID ||
          !env.FREEMIUS_PUBLIC_KEY
        ) {
          return new Response(
            "Server Misconfiguration: Missing Freemius Keys",
            { status: 500 }
          );
        }

        // Use Developer ID for scope (assuming Developer Secret Key)
        const scopeId = env.FREEMIUS_ID;

        // 1. Check License via Freemius API (Using /plugins/ endpoint)
        // Query params MUST be sorted alphabetically for signature to match!
        // count (c) comes before filter (f)
        const path = `/v1/plugins/${env.PRODUCT_ID}/licenses.json?count=1&filter=key=${licenseKey}`;
        const fsUrl = `https://api.freemius.com${path}`;

        const authHeaders = await signFreemiusRequest(
          "GET",
          path, // Sign the full path including /v1
          scopeId,
          env.FREEMIUS_PUBLIC_KEY,
          env.FREEMIUS_SECRET_KEY
        );

        const fsResp = await fetch(fsUrl, {
          method: "GET",
          headers: authHeaders,
        });

        if (!fsResp.ok) {
          const errText = await fsResp.text();
          return new Response(
            JSON.stringify({
              valid: false,
              reason: `Freemius Error ${fsResp.status}: ${errText}`,
            }),
            {
              headers: { "Content-Type": "application/json" },
            }
          );
        }

        const data = await fsResp.json();
        const licenses = data.licenses || [];

        if (licenses.length === 0) {
          return new Response(
            JSON.stringify({ valid: false, reason: "License Not Found" }),
            {
              headers: { "Content-Type": "application/json" },
            }
          );
        }

        const license = licenses[0];

        // 2. Validate Status
        const isValid = !license.is_cancelled && !license.is_expired;
        const reason = !isValid
          ? license.is_cancelled
            ? "License Cancelled"
            : "License Expired"
          : "";

        // 3. Return Result
        return new Response(
          JSON.stringify({
            valid: isValid,
            plan: license.plan_title || "Pro",
            expiry: license.expiration
              ? new Date(license.expiration).toISOString()
              : null,
            reason: reason,
          }),
          {
            headers: { "Content-Type": "application/json" },
          }
        );
      } catch (e) {
        return new Response(`Internal Error: ${e.message}`, { status: 500 });
      }
    }

    return new Response("Not Found", { status: 404 });
  },
};

/**
 * Signs a request for Freemius API using HMAC-SHA256 (Web Crypto API)
 */
async function signFreemiusRequest(
  method,
  pathURI,
  scopeId,
  publicKey,
  secretKey
) {
  const date = new Date().toUTCString();
  const contentMD5 = ""; // Empty for GET
  const contentType = "";

  // Canonical String: VERB + "\n" + Content-MD5 + "\n" + Content-Type + "\n" + Date + "\n" + Request-URI
  const stringToSign = `${method}\n${contentMD5}\n${contentType}\n${date}\n${pathURI}`;

  const encoder = new TextEncoder();
  const keyData = encoder.encode(secretKey);
  const msgData = encoder.encode(stringToSign);

  const cryptoKey = await crypto.subtle.importKey(
    "raw",
    keyData,
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"]
  );

  const signatureBuf = await crypto.subtle.sign("HMAC", cryptoKey, msgData);
  const signatureBase64 = btoa(
    String.fromCharCode(...new Uint8Array(signatureBuf))
  );

  return {
    Date: date,
    Authorization: `FS ${scopeId}:${publicKey}:${signatureBase64}`,
  };
}
