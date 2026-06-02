#!/usr/bin/env python3
"""Generate remaining AgentForge UI assets — batch 2 of 2 (78 icons)"""
import sys, os, urllib.request, time

ASSETS_DIR = os.path.expanduser("~/.openclaw/workspace/AgentForge/internal/dashboard/static/img")
API_KEY = "44ef2688-d7b6-447a-b525-dcb720431ca2:95849aeb7a1d918750438f243e1fa72e"

def gen(prompt, path):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    from fal_client import SyncClient
    c = SyncClient(key=API_KEY)
    r = c.subscribe("fal-ai/flux/schnell", {"prompt": prompt, "num_images": 1, "image_size": "square_hd"})
    urllib.request.urlretrieve(r["images"][0]["url"], path)
    t = r["timings"]["inference"]
    print(f"  ✅ {os.path.basename(path)} ({t}s)")

assets = [
    # AGENT DETAIL
    ("minimalist chat bubble icon, geometric faceted crystal shape, magma orange on black, clean UI icon, centered, no text", "icons/agent-chat.svg"),
    ("minimalist pause icon, two vertical bars geometric crystal shape, amber orange warning color, dark background, clean UI icon, centered, no text", "icons/agent-pause.svg"),
    ("minimalist stop terminate icon, square geometric crystal stop symbol, red error color, dark background, clean UI icon, centered, no text", "icons/agent-terminate.svg"),
    ("minimalist clone copy icon, two overlapping geometric squares crystal shapes, magma orange, dark background, clean UI icon, centered, no text", "icons/agent-clone.svg"),
    ("minimalist capability shield key icon, geometric crystal shield with keyhole, magma orange on black, clean UI icon, centered, no text", "icons/capability-icon.svg"),
    ("minimalist tools wrench icon, geometric faceted wrench shape, crystal glass material, magma orange on black, clean UI icon, centered, no text", "icons/tools-icon.svg"),
    ("minimalist activity pulse icon, geometric heartbeat line crystal shape, emerald green glow, dark background, clean UI icon, centered, no text", "icons/activity-icon.svg"),
    # MEMORY
    ("minimalist folder directory icon, geometric faceted folder shape, crystal glass, soft blue glow, dark background, clean UI icon, centered, no text", "icons/folder-icon.svg"),
    ("minimalist file icon, geometric rectangular crystal, soft white-blue, dark background, clean UI icon, centered, no text", "icons/file-icon.svg"),
    ("minimalist memory star icon, geometric faceted star crystal shape, gold amber glow, dark background, clean UI icon, centered, no text", "icons/file-memory.svg"),
    ("minimalist daily log icon, geometric calendar page crystal, amber warm glow, dark background, clean UI icon, centered, no text", "icons/file-daily.svg"),
    ("minimalist decision document icon, geometric crystal checkmark document, emerald green glow, dark background, clean UI icon, centered, no text", "icons/file-decision.svg"),
    ("minimalist git branch icon, geometric branch fork shape, crystal glass, soft white, dark background, clean UI icon, centered, no text", "icons/git-icon.svg"),
    ("minimalist edit pencil icon, geometric faceted pencil shape, magma orange on black, clean UI icon, centered, no text", "icons/edit-icon.svg"),
    ("minimalist preview eye icon, geometric faceted eye shape, crystal glass, soft white, dark background, clean UI icon, centered, no text", "icons/preview-icon.svg"),
    ("minimalist summarize compress icon, two arrows pointing inward geometric crystal, magenta orange, dark background, clean UI icon, centered, no text", "icons/summarize-icon.svg"),
    ("minimalist compression shrink icon, geometric shrinking square crystal, amber orange, dark background, clean UI icon, centered, no text", "icons/compress-icon.svg"),
    ("minimalist filter funnel icon, geometric faceted triangle funnel shape, magma orange on black, clean UI icon, centered, no text", "icons/filter-icon.svg"),
    ("minimalist history clock icon, geometric crystal clock face, soft white, dark background, clean UI icon, centered, no text", "icons/history-icon.svg"),
    # PIPELINES
    ("minimalist pipeline node icon, geometric hexagonal node crystal, magma orange core, dark background, clean UI icon, centered, no text", "icons/pipeline-node.svg"),
    ("minimalist connection edge arrow icon, geometric arrow crystal, magma orange line, dark background, clean UI icon, centered, no text", "icons/pipeline-edge.svg"),
    ("minimalist running play icon, geometric faceted play triangle crystal, emerald green glow, dark background, clean UI icon, centered, no text", "icons/pipeline-running.svg"),
    ("minimalist complete checkmark circle icon, geometric crystal circle with check, emerald green, dark background, clean UI icon, centered, no text", "icons/pipeline-complete.svg"),
    ("minimalist failed X mark circle icon, geometric crystal circle with X, red error, dark background, clean UI icon, centered, no text", "icons/pipeline-failed.svg"),
    ("minimalist zoom in magnifier plus icon, geometric faceted lens crystal, magma orange, dark background, clean UI icon, centered, no text", "icons/zoom-in.svg"),
    ("minimalist zoom out magnifier minus icon, geometric faceted lens crystal, magma orange, dark background, clean UI icon, centered, no text", "icons/zoom-out.svg"),
    ("minimalist fit to screen icon, geometric expanding arrows crystal shape, magma orange, dark background, clean UI icon, centered, no text", "icons/fit-screen.svg"),
    ("minimalist retry loop icon, geometric circular arrow crystal, amber orange, dark background, clean UI icon, centered, no text", "icons/stage-retry.svg"),
    ("minimalist timer clock icon, geometric crystal hourglass shape, amber orange glow, dark background, clean UI icon, centered, no text", "icons/stage-timeout.svg"),
    # SKILLS
    ("minimalist skill card icon, geometric faceted playing card crystal, magenta orange, dark background, clean UI icon, centered, no text", "icons/skill-card.svg"),
    ("minimalist download install icon, geometric arrow pointing down crystal, emerald green, dark background, clean UI icon, centered, no text", "icons/skill-install.svg"),
    ("minimalist toggle on icon, geometric crystal switch active, magma orange glow, dark background, clean UI icon, centered, no text", "icons/skill-enabled.svg"),
    ("minimalist toggle off icon, geometric crystal switch inactive, grey stone color, dark background, clean UI icon, centered, no text", "icons/skill-disabled.svg"),
    ("minimalist marketplace store icon, geometric faceted storefront crystal shape, magma orange, dark background, clean UI icon, centered, no text", "icons/marketplace-icon.svg"),
    ("minimalist trending fire icon, geometric faceted flame crystal shape, magma orange hot glow, dark background, clean UI icon, centered, no text", "icons/trending-icon.svg"),
    ("minimalist popular star icon, geometric faceted five point star crystal, gold amber glow, dark background, clean UI icon, centered, no text", "icons/popular-icon.svg"),
    # SECURITY
    ("minimalist low risk green shield icon, geometric faceted crystal shield, emerald green glow, dark background, clean UI icon, centered, no text", "icons/risk-low.svg"),
    ("minimalist medium risk amber shield icon, geometric faceted crystal shield, amber yellow glow, dark background, clean UI icon, centered, no text", "icons/risk-medium.svg"),
    ("minimalist high risk red shield icon, geometric faceted crystal shield, red error glow, dark background, clean UI icon, centered, no text", "icons/risk-high.svg"),
    ("minimalist violation warning triangle icon, geometric faceted crystal triangle with exclamation, amber orange, dark background, clean UI icon, centered, no text", "icons/violation-icon.svg"),
    ("minimalist security token key icon, geometric faceted crystal key shape, magma orange, dark background, clean UI icon, centered, no text", "icons/capability-token.svg"),
    ("minimalist expired clock icon, geometric faceted crystal clock with X, red error, dark background, clean UI icon, centered, no text", "icons/capability-expired.svg"),
    ("minimalist audit log list icon, geometric faceted scroll document crystal, soft white, dark background, clean UI icon, centered, no text", "icons/audit-log.svg"),
    ("minimalist audit allowed checkmark icon, geometric crystal circle with checkmark, emerald green glow, dark background, clean UI icon, centered, no text", "icons/audit-allow.svg"),
    ("minimalist audit denied X icon, geometric crystal circle with X, red error glow, dark background, clean UI icon, centered, no text", "icons/audit-deny.svg"),
    # SETTINGS
    ("minimalist general settings cog icon, geometric faceted gear crystal, magma orange on black, clean UI icon, centered, no text", "icons/tab-general.svg"),
    ("minimalist AI brain icon, geometric faceted hexagonal brain crystal, magenta purple, dark background, clean UI icon, centered, no text", "icons/tab-llm.svg"),
    ("minimalist memory database icon, geometric faceted stacked discs crystal, soft blue, dark background, clean UI icon, centered, no text", "icons/tab-memory.svg"),
    ("minimalist security lock icon, geometric faceted padlock crystal, amber orange, dark background, clean UI icon, centered, no text", "icons/tab-security.svg"),
    ("minimalist workers people icon, geometric faceted person silhouette crystal, soft white, dark background, clean UI icon, centered, no text", "icons/tab-workers.svg"),
    ("minimalist tools wrench icon, geometric faceted wrench crystal shape, magma orange, dark background, clean UI icon, centered, no text", "icons/tab-tools.svg"),
    ("minimalist MCP plug connector icon, geometric faceted plug shape crystal, emerald green, dark background, clean UI icon, centered, no text", "icons/tab-mcp.svg"),
    ("minimalist about info circle icon, geometric faceted circle with i, soft white, dark background, clean UI icon, centered, no text", "icons/tab-about.svg"),
    # CHAT
    ("minimalist send arrow icon, geometric faceted paper plane crystal shape, magma orange glow, dark background, clean UI icon, centered, no text", "icons/chat-send.svg"),
    ("minimalist attach paperclip icon, geometric faceted paperclip crystal shape, soft white, dark background, clean UI icon, centered, no text", "icons/chat-attach.svg"),
    ("minimalist code brackets icon, geometric faceted angle brackets crystal, emerald green, dark background, clean UI icon, centered, no text", "icons/chat-code.svg"),
    ("minimalist microphone voice icon, geometric faceted microphone crystal, magma orange, dark background, clean UI icon, centered, no text", "icons/chat-voice.svg"),
    ("minimalist token counter icon, geometric faceted hash number symbol crystal, soft white, dark background, clean UI icon, centered, no text", "icons/token-count.svg"),
    # COMMON / GENERIC
    ("minimalist close X icon, geometric faceted X mark crystal, soft white, dark background, clean UI icon, centered, no text", "icons/close-icon.svg"),
    ("minimalist expand maximize icon, geometric faceted expanding square crystal, soft white, dark background, clean UI icon, centered, no text", "icons/expand-icon.svg"),
    ("minimalist collapse minimize icon, geometric faceted shrinking square crystal, soft white, dark background, clean UI icon, centered, no text", "icons/collapse-icon.svg"),
    ("minimalist refresh reload icon, geometric faceted circular arrow crystal, magma orange, dark background, clean UI icon, centered, no text", "icons/refresh-icon.svg"),
    ("minimalist copy clipboard icon, geometric faceted overlapping pages crystal, soft white, dark background, clean UI icon, centered, no text", "icons/copy-icon.svg"),
    ("minimalist external link icon, geometric faceted square with arrow crystal, magma orange, dark background, clean UI icon, centered, no text", "icons/external-icon.svg"),
    ("minimalist chevron down arrow icon, geometric faceted V angle crystal, soft white, dark background, clean UI icon, centered, no text", "icons/chevron-down.svg"),
    ("minimalist chevron right arrow icon, geometric faceted greater than crystal, soft white, dark background, clean UI icon, centered, no text", "icons/chevron-right.svg"),
    ("minimalist plus add icon, geometric faceted plus cross crystal, magma orange, dark background, clean UI icon, centered, no text", "icons/plus-icon.svg"),
    ("minimalist minus remove icon, geometric faceted minus dash crystal, soft white, dark background, clean UI icon, centered, no text", "icons/minus-icon.svg"),
    ("minimalist checkmark icon, geometric faceted check crystal shape, emerald green, dark background, clean UI icon, centered, no text", "icons/check-icon.svg"),
    ("minimalist X cross mark icon, geometric faceted X crystal, red error, dark background, clean UI icon, centered, no text", "icons/x-icon.svg"),
    ("minimalist undo arrow icon, geometric faceted curved left arrow crystal, soft white, dark background, clean UI icon, centered, no text", "icons/undo-icon.svg"),
    ("minimalist redo arrow icon, geometric faceted curved right arrow crystal, soft white, dark background, clean UI icon, centered, no text", "icons/redo-icon.svg"),
    ("minimalist trash delete icon, geometric faceted trash bin crystal shape, red error glow, dark background, clean UI icon, centered, no text", "icons/trash-icon.svg"),
    ("minimalist download icon, geometric faceted down arrow tray crystal, emerald green, dark background, clean UI icon, centered, no text", "icons/download-icon.svg"),
    ("minimalist upload icon, geometric faceted up arrow crystal, magma orange, dark background, clean UI icon, centered, no text", "icons/upload-icon.svg"),
    ("minimalist lock icon, geometric faceted padlock closed crystal, amber orange, dark background, clean UI icon, centered, no text", "icons/lock-icon.svg"),
    ("minimalist unlock icon, geometric faceted padlock open crystal, emerald green, dark background, clean UI icon, centered, no text", "icons/unlock-icon.svg"),
    ("minimalist light mode sun icon, geometric faceted sun circle crystal, gold amber, dark background, clean UI icon, centered, no text", "icons/light-icon.svg"),
    ("minimalist dark mode moon icon, geometric faceted crescent moon crystal, soft white, dark background, clean UI icon, centered, no text", "icons/dark-icon.svg"),
]

print(f"\n{'='*60}")
print(f"🔥 AgentForge Asset Generator — Batch 2: {len(assets)} icons")
print(f"{'='*60}\n")

for i, (prompt, path) in enumerate(assets):
    name = os.path.basename(path)
    print(f"[{i+1}/{len(assets)}] {name}")
    try:
        gen(prompt, os.path.join(ASSETS_DIR, path))
    except Exception as e:
        print(f"  ❌ FAILED: {e}")
    time.sleep(0.15)

print(f"\n{'='*60}")
print(f"✅ All {len(assets)} icons complete.")
print(f"{'='*60}")