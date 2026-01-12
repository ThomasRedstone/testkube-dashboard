import pytest
from playwright.sync_api import Page, expect

def test_workflow_list(page: Page):
    # 1. Arrange: Go to the workflows page.
    page.goto("http://localhost:8080/workflows")

    # 2. Assert: Check title
    expect(page).to_have_title("Testkube Dashboard")

    # 3. Assert: Check for workflows table
    expect(page.get_by_text("Test Workflows")).to_be_visible()
    expect(page.locator("table")).to_be_visible()

    # 4. Assert: Check for a specific workflow
    expect(page.get_by_text("texecom-e2e-tests-eks")).to_be_visible()

def test_workflow_history(page: Page):
    # 1. Arrange: Go to the workflows page.
    page.goto("http://localhost:8080/workflows")

    # 2. Act: Click on the history link for the texecom-e2e-tests-eks workflow
    page.click("a[href='/workflows/texecom-e2e-tests-eks/history']")

    # 3. Assert: We are on the workflow history page
    expect(page).to_have_url("http://localhost:8080/workflows/texecom-e2e-tests-eks/history")
    expect(page.get_by_text("Execution History for texecom-e2e-tests-eks")).to_be_visible()

    # 4. Assert: Check for the execution history table
    expect(page.locator("table")).to_be_visible()