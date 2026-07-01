#!/bin/bash

echo "Starting SonarQube server..."
docker compose --profile sonar up -d sonarqube

echo "========================================================"
echo "🚀 Step 1: Building and Running Tests & Security Scans"
echo "========================================================"
# This builds the test containers and runs all the tests, linters, and vulnerability scans.
# We explicitly specify test containers so `docker compose up` exits when they finish,
# rather than hanging on long-running default services (like postgres).
docker compose --profile test up --build backend-test web-test web-shop-test

TEST_EXIT_CODE=0
for svc in backend-test web-test web-shop-test; do
    CID=$(docker compose --profile test ps -a -q $svc)
    if [ ! -z "$CID" ]; then
        code=$(docker inspect -f '{{.State.ExitCode}}' $CID)
        if [ "$code" != "0" ]; then
            TEST_EXIT_CODE=1
        fi
    fi
done

echo ""
echo "========================================================"
echo "📡 Step 2: Publishing Reports to SonarQube Dashboard"
echo "========================================================"
# Run the sonar scanner to upload the generated reports in ./reports.
#
# SonarScanner v8+ (the :latest image) dropped password auth — it strictly
# requires a token. The token API also 503s until SonarQube finishes booting,
# so poll /api/system/status for "status":"UP" before doing anything.

echo "⏳ Waiting for SonarQube to be UP (Elasticsearch boot can take a minute)..."
SONAR_STATUS=""
for i in $(seq 1 60); do
    SONAR_STATUS=$(curl -s http://localhost:9000/api/system/status | grep -o '"status":"[^"]*' | grep -o '[^"]*$')
    if [ "$SONAR_STATUS" = "UP" ]; then
        echo "✅ SonarQube is UP."
        break
    fi
    echo "   ...still booting (status=${SONAR_STATUS:-unreachable}) [$i/60]"
    sleep 5
done

if [ "$SONAR_STATUS" != "UP" ]; then
    echo "❌ SonarQube did not become healthy in time (5 min). Skipping publish."
    echo "   Check logs: docker compose --profile sonar logs sonarqube"
elif [ -n "$SONAR_TOKEN" ]; then
    # Caller supplied a token explicitly — use it.
    docker compose --profile sonar run -e SONAR_TOKEN="${SONAR_TOKEN}" sonar-scanner
else
    echo "Generating temporary SonarQube token..."
    # Generate token using default admin:admin (fails gracefully if password was changed)
    TOKEN_JSON=$(curl -s -u admin:admin -X POST "http://localhost:9000/api/user_tokens/generate?name=auto-scanner-$(date +%s)&login=admin")
    AUTO_TOKEN=$(echo $TOKEN_JSON | grep -o '"token":"[^"]*' | grep -o '[^"]*$')

    if [ -n "$AUTO_TOKEN" ]; then
        docker compose --profile sonar run -e SONAR_TOKEN="${AUTO_TOKEN}" sonar-scanner
    else
        echo "⚠️  Failed to auto-generate token (did you change the admin password?)."
        echo "Please generate a token manually and run: export SONAR_TOKEN=your_token && ./scripts/scan.sh"
    fi
fi

echo ""
echo "========================================================"
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo "✅ Done! All scans passed successfully."
else
    echo "❌ Done! Some tests or scans failed (which is normal in TDD)."
fi
echo "👉 Open your dashboard to view the results: http://localhost:9000"
echo "   (Note: It may take a minute or two for SonarQube to boot up completely on first run)"
echo "========================================================"

exit $TEST_EXIT_CODE
