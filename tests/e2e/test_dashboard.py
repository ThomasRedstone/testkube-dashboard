import pytest
from playwright.sync_api import Page, expect

def test_dashboard_summary(page: Page):
    # 1. Arrange: Go to the Dashboard homepage.
    page.goto("http://localhost:8080")

    # 2. Assert: Check title
    expect(page).to_have_title("Testkube Dashboard")

    # 3. Assert: Check for Dashboard cards
    expect(page.get_by_text("Total Tests")).to_be_visible()
    expect(page.get_by_text("Pass Rate")).to_be_visible()
    expect(page.get_by_text("Total Executions")).to_be_visible()

    # Check values (based on mock data)
    # Total Tests: 3
    expect(page.get_by_text("3", exact=True)).to_be_visible()

    # Pass Rate: 50.0%
    expect(page.get_by_text("50.0%")).to_be_visible()

    # Recent Failures section
    expect(page.get_by_text("Recent Failures")).to_be_visible()
    # Should see "api-sanity-check" in the table
    expect(page.locator(".recent-failures table")).to_contain_text("api-sanity-check")

def test_navigation_to_tests(page: Page):
    page.goto("http://localhost:8080")

    # Act: Click on "Tests" in nav
    page.click("text=Tests")

    # Assert: We are on the test list page
    # Since index.html uses htmx to load tests, we wait for the table
    page.wait_for_selector("table")
    expect(page.locator("table")).to_be_visible()

    # Verify content of test list
    expect(page.get_by_text("api-sanity-check")).to_be_visible()
