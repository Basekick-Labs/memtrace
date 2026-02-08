#!/usr/bin/env python3
"""TeleCo Customer Support — 3 Telegram Bots with Shared Memtrace Memory

Three Telegram bots acting as customer support departments for a fictional
telco "TeleCo". A customer chats with one bot about an internet issue, then
contacts another about TV, then billing — each bot sees the full history
from all departments via Memtrace shared sessions.

Bots:
  - Internet Support  (BOT_TOKEN_INTERNET)
  - TV Support        (BOT_TOKEN_TV)
  - Billing           (BOT_TOKEN_BILLING)

Demonstrates Memtrace's core value: cross-agent memory sharing for AI
customer support.

Setup:
  pip install -r requirements.txt

  export ANTHROPIC_API_KEY="sk-ant-..."
  export MEMTRACE_URL="http://localhost:9100"
  export MEMTRACE_API_KEY="mtk_..."
  export BOT_TOKEN_INTERNET="<telegram bot token for internet support>"
  export BOT_TOKEN_TV="<telegram bot token for tv support>"
  export BOT_TOKEN_BILLING="<telegram bot token for billing>"

  python telegram_support.py
"""

from __future__ import annotations

import asyncio
import base64
import json
import os
import sys
import time
from dataclasses import dataclass, field

import anthropic
from memtrace import (
    AddMemoryRequest,
    ConflictError,
    ContextOptions,
    CreateSessionRequest,
    ListOptions,
    Memtrace,
    RegisterAgentRequest,
    SearchQuery,
)
from telegram import Update
from telegram.constants import ChatAction
from telegram.ext import Application, MessageHandler, filters

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

ANTHROPIC_API_KEY = os.environ.get("ANTHROPIC_API_KEY", "")
MEMTRACE_URL = os.environ.get("MEMTRACE_URL", "http://localhost:9100")
MEMTRACE_API_KEY = os.environ.get("MEMTRACE_API_KEY", "")

BOT_TOKEN_INTERNET = os.environ.get("BOT_TOKEN_INTERNET", "")
BOT_TOKEN_TV = os.environ.get("BOT_TOKEN_TV", "")
BOT_TOKEN_BILLING = os.environ.get("BOT_TOKEN_BILLING", "")

MODEL = "claude-sonnet-4-20250514"
MAX_TOOL_ROUNDS = 15

# ---------------------------------------------------------------------------
# Test customer data
# ---------------------------------------------------------------------------

TEST_CUSTOMERS: dict[str, dict] = {
    "nacho": {
        "first_name": "Nacho",
        "last_name": "Bassino",
        "address": "742 Evergreen Terrace, Springfield, IL 62704",
        "phone": "+1-555-0123",
        "email": "nacho@example.com",
        "dob": "1990-03-15",
        "account_id": "TC-100001",
        "plan": "TeleCo Premium Bundle (500 Mbps Internet + TV Gold + Phone)",
        "monthly_bill": "$189.99",
        "member_since": "2021-06-01",
    },
    "maria": {
        "first_name": "Maria",
        "last_name": "Garcia",
        "address": "1600 Pennsylvania Ave, Washington, DC 20500",
        "phone": "+1-555-0456",
        "email": "maria@example.com",
        "dob": "1985-07-22",
        "account_id": "TC-100002",
        "plan": "TeleCo Basic Internet (100 Mbps)",
        "monthly_bill": "$49.99",
        "member_since": "2023-01-15",
    },
    "alex": {
        "first_name": "Alex",
        "last_name": "Johnson",
        "address": "221B Baker Street, London, UK",
        "phone": "+44-7700-900123",
        "email": "alex@example.com",
        "dob": "1992-11-08",
        "account_id": "TC-100003",
        "plan": "TeleCo TV & Internet (200 Mbps + TV Silver)",
        "monthly_bill": "$129.99",
        "member_since": "2022-09-01",
    },
}


def lookup_customer(name: str) -> dict | None:
    """Case-insensitive first-name lookup."""
    return TEST_CUSTOMERS.get(name.strip().lower())


# ---------------------------------------------------------------------------
# Bot persona configs
# ---------------------------------------------------------------------------


@dataclass
class BotConfig:
    name: str          # Display name (e.g. "Juan")
    agent_name: str    # Memtrace agent name (e.g. "internet-support")
    department: str    # Department label
    token: str         # Telegram bot token
    system_prompt: str # Base system prompt for this bot


BOT_CONFIGS = [
    BotConfig(
        name="Juan",
        agent_name="internet-support",
        department="Internet Support",
        token=BOT_TOKEN_INTERNET,
        system_prompt=(
            "You are Juan, a friendly and knowledgeable Internet Support specialist "
            "at TeleCo, a telecommunications company.\n\n"
            "Your expertise:\n"
            "- Internet connectivity issues (slow speeds, outages, DNS problems)\n"
            "- Router/modem troubleshooting\n"
            "- Network diagnostics and optimization\n"
            "- Plan upgrades for internet service\n\n"
            "Guidelines:\n"
            "- Be warm, professional, and empathetic\n"
            "- Use memtrace_remember to store every customer issue, action taken, and resolution\n"
            "- Use memtrace_recall and memtrace_search to check if this customer has contacted "
            "other departments — acknowledge their prior interactions\n"
            "- Tag memories with relevant keywords using hyphens, no spaces "
            "(internet, troubleshooting, slow-speed, outage, router, dns)\n"
            "- Set importance scores based on issue severity (0.0-1.0)\n"
            "- If you see memories from other departments (tv-support, billing), "
            "reference them naturally: 'I see you also spoke with our TV team about...'\n"
            "- Keep responses concise but helpful (2-4 sentences typical)\n"
            "- Always store the issue and any resolution as episodic memories\n"
            "- IMPORTANT: When you make a decision (credit, discount, escalation), your reply to the "
            "customer MUST include the specific details in your message text: exact amount, duration, "
            "what it applies to, and when it takes effect. Do NOT just log it as a tool call — "
            "the customer needs to see it in your response"
        ),
    ),
    BotConfig(
        name="Martin",
        agent_name="tv-support",
        department="TV Support",
        token=BOT_TOKEN_TV,
        system_prompt=(
            "You are Martin, a friendly and knowledgeable TV Support specialist "
            "at TeleCo, a telecommunications company.\n\n"
            "Your expertise:\n"
            "- TV service issues (channels missing, picture quality, DVR problems)\n"
            "- Set-top box troubleshooting\n"
            "- Channel package upgrades and add-ons (HBO, sports packages)\n"
            "- Streaming and on-demand service issues\n\n"
            "Guidelines:\n"
            "- Be warm, professional, and empathetic\n"
            "- Use memtrace_remember to store every customer issue, action taken, and resolution\n"
            "- Use memtrace_recall and memtrace_search to check if this customer has contacted "
            "other departments — acknowledge their prior interactions\n"
            "- Tag memories with relevant keywords using hyphens, no spaces "
            "(tv, channels, set-top-box, dvr, streaming, hbo, sports-package)\n"
            "- Set importance scores based on issue severity (0.0-1.0)\n"
            "- If you see memories from other departments (internet-support, billing), "
            "reference them naturally: 'I see you also spoke with our Internet team about...'\n"
            "- Keep responses concise but helpful (2-4 sentences typical)\n"
            "- Always store the issue and any resolution as episodic memories\n"
            "- IMPORTANT: When you make a decision (credit, discount, escalation), your reply to the "
            "customer MUST include the specific details in your message text: exact amount, duration, "
            "what it applies to, and when it takes effect. Do NOT just log it as a tool call — "
            "the customer needs to see it in your response"
        ),
    ),
    BotConfig(
        name="Cecilia",
        agent_name="billing",
        department="Billing",
        token=BOT_TOKEN_BILLING,
        system_prompt=(
            "You are Cecilia, a friendly and knowledgeable Billing specialist "
            "at TeleCo, a telecommunications company.\n\n"
            "Your expertise:\n"
            "- Billing inquiries (charges, credits, payment history)\n"
            "- Plan changes and pricing\n"
            "- Payment methods and autopay setup\n"
            "- Disputes and credit adjustments\n"
            "- Promotional offers and discounts\n\n"
            "Guidelines:\n"
            "- Be warm, professional, and empathetic\n"
            "- Use memtrace_remember to store every customer issue, action taken, and resolution\n"
            "- Use memtrace_recall and memtrace_search to check if this customer has contacted "
            "other departments — acknowledge their prior interactions\n"
            "- Tag memories with relevant keywords using hyphens, no spaces "
            "(billing, charges, payment, credit, plan-change, discount, dispute)\n"
            "- Set importance scores based on issue severity (0.0-1.0)\n"
            "- If you see memories from other departments (internet-support, tv-support), "
            "reference them naturally: 'I see you also contacted our Internet team about...'\n"
            "- If a customer had a service issue with another department, proactively offer "
            "a credit or discount as goodwill\n"
            "- Keep responses concise but helpful (2-4 sentences typical)\n"
            "- Always store the issue and any resolution as episodic memories\n"
            "- IMPORTANT: When you make a decision (credit, discount, escalation), your reply to the "
            "customer MUST include the specific details in your message text: exact amount, duration, "
            "what it applies to, and when it takes effect. Do NOT just log it as a tool call — "
            "the customer needs to see it in your response"
        ),
    ),
]

# ---------------------------------------------------------------------------
# User session state (shared across all 3 bots)
# ---------------------------------------------------------------------------


@dataclass
class UserSession:
    telegram_user_id: int
    session_id: str
    customer: dict
    verified: bool = False
    conversations: dict[str, list] = field(default_factory=dict)
    # conversations keyed by agent_name → list of {"role": ..., "content": ...}


# Shared across all bots — keyed by telegram_user_id
USER_SESSIONS: dict[int, UserSession] = {}

# ---------------------------------------------------------------------------
# Memtrace tool definitions (same as superbowl demo)
# ---------------------------------------------------------------------------

MEMTRACE_TOOLS = [
    {
        "name": "memtrace_remember",
        "description": (
            "Store a memory. Use this to record customer issues, actions taken, "
            "resolutions, and any information worth remembering for future interactions."
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "content": {
                    "type": "string",
                    "description": "Memory content text",
                },
                "memory_type": {
                    "type": "string",
                    "description": "Memory type: episodic (default), decision, entity, session",
                    "enum": ["episodic", "decision", "entity", "session"],
                },
                "event_type": {
                    "type": "string",
                    "description": "Event type (e.g. observation, action, error, resolution). Default: general",
                },
                "tags": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Tags for categorization (use hyphens, no spaces)",
                },
                "importance": {
                    "type": "number",
                    "description": "Importance score 0.0 to 1.0",
                },
            },
            "required": ["content"],
        },
    },
    {
        "name": "memtrace_recall",
        "description": (
            "Retrieve recent memories from ALL departments for this customer session. "
            "Returns memories in reverse chronological order. Use this to check if the "
            "customer has contacted other departments."
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "since": {
                    "type": "string",
                    "description": "Time window (e.g. 2h, 24h, 7d). Default: 24h",
                },
                "memory_type": {
                    "type": "string",
                    "description": "Filter by memory type",
                    "enum": ["episodic", "decision", "entity", "session"],
                },
                "limit": {
                    "type": "integer",
                    "description": "Max results. Default: 50",
                },
            },
            "required": [],
        },
    },
    {
        "name": "memtrace_search",
        "description": (
            "Search memories across ALL departments for this customer session. "
            "Use structured filters: content text, memory types, tags, importance, "
            "and time range."
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "content_contains": {
                    "type": "string",
                    "description": "Search text within memory content",
                },
                "memory_types": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Filter by memory types (episodic, decision, entity, session)",
                },
                "tags": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Filter by tags",
                },
                "since": {
                    "type": "string",
                    "description": "Time window (e.g. 2h, 24h)",
                },
                "min_importance": {
                    "type": "number",
                    "description": "Minimum importance score 0.0 to 1.0",
                },
                "limit": {
                    "type": "integer",
                    "description": "Max results. Default: 50",
                },
            },
            "required": [],
        },
    },
    {
        "name": "memtrace_decide",
        "description": (
            "Log a decision with reasoning. Creates an auditable record of what was "
            "decided and why (e.g. issuing a credit, escalating an issue)."
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "decision": {
                    "type": "string",
                    "description": "The decision that was made",
                },
                "reasoning": {
                    "type": "string",
                    "description": "Why this decision was made",
                },
            },
            "required": ["decision", "reasoning"],
        },
    },
]

# ---------------------------------------------------------------------------
# Tool handler (cross-bot: writes with agent_id, reads by session_id)
# ---------------------------------------------------------------------------


def handle_tool_call(
    tool_name: str,
    tool_input: dict,
    mt: Memtrace,
    agent_id: str,
    session_id: str,
) -> str:
    """Execute a Memtrace tool call. Writes use agent_id for attribution,
    reads use session_id so every bot sees all departments' memories."""
    try:
        if tool_name == "memtrace_remember":
            mem = mt.add_memory(
                AddMemoryRequest(
                    agent_id=agent_id,
                    session_id=session_id,
                    content=tool_input["content"],
                    memory_type=tool_input.get("memory_type", "episodic"),
                    event_type=tool_input.get("event_type", "general"),
                    tags=tool_input.get("tags"),
                    importance=tool_input.get("importance"),
                )
            )
            return json.dumps(
                {"stored": True, "content": mem.content, "time": str(mem.time)}
            )

        if tool_name == "memtrace_recall":
            # Query by session_id (not agent_id) to see ALL departments
            result = mt.list_memories(
                ListOptions(
                    session_id=session_id,
                    since=tool_input.get("since", "24h"),
                    memory_type=tool_input.get("memory_type"),
                    limit=min(tool_input.get("limit", 50), 200),
                    order="desc",
                )
            )
            memories = [
                {
                    "time": str(m.time),
                    "agent": m.agent_id,
                    "content": m.content,
                    "type": m.memory_type,
                    "tags": m.tags,
                }
                for m in result.memories
            ]
            return json.dumps({"count": result.count, "memories": memories})

        if tool_name == "memtrace_search":
            # Query by session_id (not agent_id) to see ALL departments
            result = mt.search_memories(
                SearchQuery(
                    session_id=session_id,
                    content_contains=tool_input.get("content_contains"),
                    memory_types=tool_input.get("memory_types"),
                    tags=tool_input.get("tags"),
                    since=tool_input.get("since"),
                    min_importance=tool_input.get("min_importance"),
                    limit=min(tool_input.get("limit", 50), 200),
                    order="desc",
                )
            )
            results = [
                {
                    "time": str(m.time),
                    "agent": m.agent_id,
                    "content": m.content,
                    "type": m.memory_type,
                    "tags": m.tags,
                }
                for m in result.results
            ]
            return json.dumps({"count": result.count, "results": results})

        if tool_name == "memtrace_decide":
            mem = mt.decide(agent_id, tool_input["decision"], tool_input["reasoning"])
            return json.dumps(
                {"logged": True, "decision": mem.content, "time": str(mem.time)}
            )

        return json.dumps({"error": f"Unknown tool: {tool_name}"})
    except Exception as exc:
        return json.dumps({"error": str(exc)})


# ---------------------------------------------------------------------------
# Agentic loop (sync — runs in thread executor from async handlers)
# ---------------------------------------------------------------------------


def run_agent_loop(
    claude: anthropic.Anthropic,
    mt: Memtrace,
    system_prompt: str,
    messages: list[dict],
    agent_id: str,
    session_id: str,
) -> str:
    """Run the Claude tool-use loop until the model stops calling tools."""
    last_text = ""

    for _round in range(MAX_TOOL_ROUNDS):
        # Retry with exponential backoff on rate limits
        for attempt in range(5):
            try:
                response = claude.messages.create(
                    model=MODEL,
                    max_tokens=2048,
                    system=system_prompt,
                    tools=MEMTRACE_TOOLS,
                    messages=messages,
                )
                break
            except anthropic.RateLimitError:
                wait = 2**attempt * 10  # 10s, 20s, 40s, 80s, 160s
                print(f"  [rate limited] waiting {wait}s before retry ({attempt + 1}/5)")
                time.sleep(wait)
                if attempt == 4:
                    raise

        # Extract any text from this response
        text_parts = [
            block.text for block in response.content if block.type == "text"
        ]
        if text_parts:
            last_text = "\n".join(text_parts).strip()

        # Collect tool calls
        tool_calls = [b for b in response.content if b.type == "tool_use"]

        # Process tool calls and build results
        tool_results = []
        for block in tool_calls:
            print(f"  [{agent_id}] tool: {block.name}({json.dumps(block.input)[:80]})")
            result = handle_tool_call(
                block.name, block.input, mt, agent_id, session_id
            )
            tool_results.append(
                {
                    "type": "tool_result",
                    "tool_use_id": block.id,
                    "content": result,
                }
            )

        # Append assistant message (only if non-empty content)
        if response.content:
            messages.append({"role": "assistant", "content": response.content})

        if response.stop_reason == "end_turn":
            if last_text:
                return last_text
            # Claude ended without any text — nudge for a customer-facing reply
            messages.append(
                {"role": "user", "content": "Please provide your response to the customer."}
            )
            continue

        # Feed tool results back
        if tool_results:
            messages.append({"role": "user", "content": tool_results})
        else:
            break

    return last_text or "[No response generated]"


# ---------------------------------------------------------------------------
# Agent registration helper
# ---------------------------------------------------------------------------


def register_agent(mt: Memtrace, name: str, description: str) -> str:
    """Register an agent, returning its ID. Idempotent."""
    try:
        agent = mt.register_agent(
            RegisterAgentRequest(name=name, description=description)
        )
    except ConflictError:
        agent = mt.get_agent(name)
    return agent.id


def find_active_session(mt: Memtrace, agent_id: str, account_id: str) -> str | None:
    """Find an active session for a customer by account_id in session metadata.
    Searches across all agents' sessions to find one matching this customer."""
    resp = mt._client.get("/api/v1/sessions", params={"agent_id": agent_id})
    if resp.status_code != 200:
        return None
    data = resp.json()
    for s in data.get("sessions", []):
        if (
            s.get("status") == "active"
            and s.get("metadata", {}).get("account_id") == account_id
        ):
            return s["id"]
    return None


def find_active_session_any_agent(
    mt: Memtrace, agent_ids: dict[str, str], account_id: str
) -> str | None:
    """Search across all agents for an active session matching the customer."""
    for _, aid in agent_ids.items():
        session_id = find_active_session(mt, aid, account_id)
        if session_id:
            return session_id
    return None


# ---------------------------------------------------------------------------
# Telegram message handler factory
# ---------------------------------------------------------------------------


def make_message_handler(
    bot_config: BotConfig,
    mt: Memtrace,
    claude: anthropic.Anthropic,
    agent_ids: dict[str, str],
    all_agent_ids: dict[str, str],
):
    """Create an async Telegram message handler for a specific bot."""

    agent_id = agent_ids[bot_config.agent_name]

    async def handler(update: Update, context) -> None:
        if not update.message:
            return

        has_text = bool(update.message.text)
        has_photo = bool(update.message.photo)
        if not has_text and not has_photo:
            return

        user_id = update.effective_user.id
        user_text = (update.message.text or "").strip()
        loop = asyncio.get_event_loop()

        # Show typing indicator
        await update.message.chat.send_action(ChatAction.TYPING)

        # --- Verification flow ---
        session = USER_SESSIONS.get(user_id)

        if session is None:
            # First contact — ask for name
            customer = lookup_customer(user_text)
            if customer is None:
                await update.message.reply_text(
                    f"Hello! I'm {bot_config.name} from TeleCo {bot_config.department}. "
                    f"To help you, I need to verify your identity.\n\n"
                    f"Could you please tell me your first name?"
                )
                return

            # Name matched — check for existing active session or create new one
            print(f"[{bot_config.name}] Customer verified: {customer['first_name']} {customer['last_name']}")

            existing_session_id = await loop.run_in_executor(
                None,
                lambda: find_active_session_any_agent(mt, all_agent_ids, customer["account_id"]),
            )

            if existing_session_id:
                print(f"[{bot_config.name}] Resuming existing session: {existing_session_id}")
                session_id = existing_session_id
            else:
                mt_session = await loop.run_in_executor(
                    None,
                    lambda: mt.create_session(
                        CreateSessionRequest(
                            agent_id=agent_id,
                            metadata={
                                "customer_name": f"{customer['first_name']} {customer['last_name']}",
                                "account_id": customer["account_id"],
                                "channel": "telegram",
                            },
                        )
                    ),
                )
                session_id = mt_session.id
                print(f"[{bot_config.name}] Created new session: {session_id}")

            session = UserSession(
                telegram_user_id=user_id,
                session_id=session_id,
                customer=customer,
                verified=True,
            )
            USER_SESSIONS[user_id] = session

            # Store entity memory with customer profile (ignore duplicate from prior runs)
            try:
                await loop.run_in_executor(
                    None,
                    lambda: mt.add_memory(
                        AddMemoryRequest(
                            agent_id=agent_id,
                            session_id=session.session_id,
                            content=(
                                f"Customer verified: {customer['first_name']} {customer['last_name']}\n"
                                f"Account: {customer['account_id']}\n"
                                f"Plan: {customer['plan']}\n"
                                f"Monthly bill: {customer['monthly_bill']}\n"
                                f"Member since: {customer['member_since']}\n"
                                f"Address: {customer['address']}\n"
                                f"Phone: {customer['phone']}\n"
                                f"Email: {customer['email']}"
                            ),
                            memory_type="entity",
                            event_type="customer_verification",
                            tags=["customer-profile", "verification"],
                            importance=1.0,
                        )
                    ),
                )
            except ConflictError:
                print(f"[{bot_config.name}] Customer profile already stored (dedup), continuing")

            await update.message.reply_text(
                f"Welcome, {customer['first_name']}! I've verified your identity. "
                f"I'm {bot_config.name} from TeleCo {bot_config.department}. "
                f"How can I help you today?"
            )
            return

        if not session.verified:
            # Shouldn't happen with current flow, but guard
            customer = lookup_customer(user_text)
            if customer is None:
                await update.message.reply_text(
                    "I couldn't find that name in our system. "
                    "Could you try your first name again?"
                )
                return
            session.customer = customer
            session.verified = True

        # --- Chat flow (verified customer) ---
        await update.message.chat.send_action(ChatAction.TYPING)

        # Load session context from Memtrace (sees ALL departments)
        ctx = await loop.run_in_executor(
            None,
            lambda: mt.get_session_context(
                session.session_id, ContextOptions(since="24h")
            ),
        )

        # Build full system prompt with customer profile + cross-department context
        customer = session.customer
        full_system = (
            f"{bot_config.system_prompt}\n\n"
            f"## Customer Profile\n"
            f"- Name: {customer['first_name']} {customer['last_name']}\n"
            f"- Account: {customer['account_id']}\n"
            f"- Plan: {customer['plan']}\n"
            f"- Monthly bill: {customer['monthly_bill']}\n"
            f"- Member since: {customer['member_since']}\n"
            f"- Address: {customer['address']}\n"
            f"- Phone: {customer['phone']}\n"
            f"- Email: {customer['email']}\n"
        )

        if ctx.context and ctx.memory_count > 0:
            full_system += (
                f"\n## Prior Interactions (from ALL departments)\n"
                f"{ctx.context}"
            )

        # Get or initialize conversation for this bot
        if bot_config.agent_name not in session.conversations:
            session.conversations[bot_config.agent_name] = []
        convo = session.conversations[bot_config.agent_name]

        # Build user message content (text, photo, or both)
        user_content: list[dict] | str = user_text or ""

        if has_photo:
            # Download the largest photo resolution
            photo = update.message.photo[-1]  # highest res
            photo_file = await photo.get_file()
            photo_bytes = await photo_file.download_as_bytearray()
            photo_b64 = base64.standard_b64encode(bytes(photo_bytes)).decode("utf-8")

            # Build multimodal content blocks
            content_blocks: list[dict] = [
                {
                    "type": "image",
                    "source": {
                        "type": "base64",
                        "media_type": "image/jpeg",
                        "data": photo_b64,
                    },
                },
            ]
            # Add caption text if present
            caption = (update.message.caption or "").strip()
            if caption:
                content_blocks.append({"type": "text", "text": caption})
            elif user_text:
                content_blocks.append({"type": "text", "text": user_text})
            else:
                content_blocks.append({"type": "text", "text": "The customer sent this photo."})

            user_content = content_blocks
            print(f"[{bot_config.name}] Photo received from user {user_id} ({len(photo_bytes)} bytes)")

        convo.append({"role": "user", "content": user_content})

        # Run agentic loop in executor (sync Claude + Memtrace calls)
        log_text = user_text or (update.message.caption or "[photo]")
        print(f"[{bot_config.name}] Processing message from user {user_id}: {log_text[:50]}...")

        reply = await loop.run_in_executor(
            None,
            lambda: run_agent_loop(
                claude, mt, full_system, list(convo),
                agent_id, session.session_id,
            ),
        )

        # Update conversation with assistant response
        convo.append({"role": "assistant", "content": reply})

        # Guard against empty reply
        if not reply or not reply.strip():
            reply = "I'm sorry, I encountered an issue processing your request. Could you please try again?"

        # Send reply (Telegram has a 4096 char limit per message)
        if len(reply) <= 4096:
            await update.message.reply_text(reply)
        else:
            for i in range(0, len(reply), 4096):
                await update.message.reply_text(reply[i : i + 4096])

        print(f"[{bot_config.name}] Replied to user {user_id}")

    return handler


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


async def main() -> None:
    # Validate env vars
    missing = []
    if not ANTHROPIC_API_KEY:
        missing.append("ANTHROPIC_API_KEY")
    if not MEMTRACE_API_KEY:
        missing.append("MEMTRACE_API_KEY")
    if not BOT_TOKEN_INTERNET:
        missing.append("BOT_TOKEN_INTERNET")
    if not BOT_TOKEN_TV:
        missing.append("BOT_TOKEN_TV")
    if not BOT_TOKEN_BILLING:
        missing.append("BOT_TOKEN_BILLING")
    if missing:
        print(f"Error: missing environment variables: {', '.join(missing)}")
        sys.exit(1)

    # Initialize clients
    claude = anthropic.Anthropic(api_key=ANTHROPIC_API_KEY)
    mt = Memtrace(MEMTRACE_URL, MEMTRACE_API_KEY)

    try:
        # Register agents
        print("1. Registering agents...")
        agent_ids: dict[str, str] = {}
        for cfg in BOT_CONFIGS:
            aid = register_agent(mt, cfg.agent_name, f"TeleCo {cfg.department}")
            agent_ids[cfg.agent_name] = aid
            print(f"   {cfg.name} ({cfg.agent_name}): {aid}")

        # Build Telegram applications
        print("2. Starting Telegram bots...")
        apps: list[Application] = []
        for cfg in BOT_CONFIGS:
            app = Application.builder().token(cfg.token).build()
            app.add_handler(
                MessageHandler(
                    (filters.TEXT | filters.PHOTO) & ~filters.COMMAND,
                    make_message_handler(cfg, mt, claude, agent_ids, agent_ids),
                )
            )
            apps.append(app)
            print(f"   {cfg.name} ({cfg.department}) — ready")

        # Start all bots using manual lifecycle
        for app in apps:
            await app.initialize()
            await app.start()
            await app.updater.start_polling()

        print("\n" + "=" * 60)
        print("All 3 TeleCo support bots are running!")
        print("=" * 60)
        print("\nBots:")
        for cfg in BOT_CONFIGS:
            print(f"  - {cfg.name} ({cfg.department})")
        print("\nTest customers: nacho, maria, alex")
        print("Press Ctrl+C to stop.\n")

        # Wait forever until interrupted
        stop_event = asyncio.Event()
        try:
            await stop_event.wait()
        except asyncio.CancelledError:
            pass

    except KeyboardInterrupt:
        pass
    finally:
        # Graceful shutdown
        print("\nShutting down...")
        for app in apps:
            try:
                await app.updater.stop()
                await app.stop()
                await app.shutdown()
            except Exception:
                pass
        mt.close()
        print("Done.")


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass
