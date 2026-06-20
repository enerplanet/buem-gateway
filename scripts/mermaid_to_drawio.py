#!/usr/bin/env python3
"""Convert a limited Mermaid subset used in this repo into draw.io files."""

from __future__ import annotations

import argparse
import datetime as dt
import html
import re
import uuid
import xml.etree.ElementTree as ET
from dataclasses import dataclass, field
from pathlib import Path


NODE_ID_RE = re.compile(r"^[A-Za-z0-9_]+")
PARTICIPANT_RE = re.compile(r"^participant\s+(\w+)\s+as\s+(.+)$")
MESSAGE_RE = re.compile(r"^(\w+)\s*([<-]+>{1,2}|-->>|->>)\s*(\w+):\s*(.+)$")
NOTE_RE = re.compile(r"^Note\s+over\s+(\w+)(?:,(\w+))?:\s*(.+)$")


@dataclass
class Node:
    """A Mermaid node rendered as a draw.io shape."""

    node_id: str
    label: str
    shape: str = "rect"
    classes: list[str] = field(default_factory=list)
    style: dict[str, str] = field(default_factory=dict)
    x: int = 0
    y: int = 0
    width: int = 0
    height: int = 0


@dataclass
class Group:
    """A Mermaid subgraph rendered as a background container."""

    group_id: str
    label: str
    direction: str = "TB"
    items: list[tuple[str, str]] = field(default_factory=list)
    style: dict[str, str] = field(default_factory=dict)
    x: int = 0
    y: int = 0
    width: int = 0
    height: int = 0
    rel_positions: dict[str, tuple[int, int]] = field(default_factory=dict)


@dataclass
class Edge:
    """A Mermaid edge rendered as a draw.io connector."""

    source: str
    target: str
    label: str = ""
    dashed: bool = False
    bidirectional: bool = False
    undirected: bool = False


@dataclass
class FlowDiagram:
    """A parsed Mermaid flowchart or graph."""

    direction: str
    nodes: dict[str, Node]
    groups: dict[str, Group]
    edges: list[Edge]
    root_group: str


@dataclass
class SequenceParticipant:
    """A Mermaid sequence participant."""

    alias: str
    label: str


@dataclass
class SequenceEvent:
    """A sequence event with a fixed vertical slot."""

    event_type: str
    source: str | None = None
    target: str | None = None
    label: str = ""
    dashed: bool = False
    span_start: str | None = None
    span_end: str | None = None
    block_id: str | None = None
    y: int = 0
    height: int = 0


@dataclass
class SequenceBlock:
    """Background region for a sequence phase."""

    block_id: str
    color: str
    start_y: int = 0
    end_y: int = 0


@dataclass
class SequenceDiagram:
    """A parsed Mermaid sequence diagram."""

    participants: list[SequenceParticipant]
    events: list[SequenceEvent]
    blocks: list[SequenceBlock]


def strip_frontmatter(text: str) -> str:
    """Remove Mermaid YAML frontmatter blocks."""

    lines = text.splitlines()
    if lines and lines[0].strip() == "---":
        for index in range(1, len(lines)):
            if lines[index].strip() == "---":
                return "\n".join(lines[index + 1 :])
    return text


def normalize_flow_lines(text: str) -> list[str]:
    """Collapse multiline Mermaid statements used inside quoted node labels."""

    normalized: list[str] = []
    buffer = ""
    for raw_line in strip_frontmatter(text).splitlines():
        line = raw_line.strip()
        if not line:
            continue
        if buffer:
            buffer = f"{buffer}\\n{line}"
        else:
            buffer = line
        if buffer.count('"') % 2 == 0:
            normalized.append(buffer)
            buffer = ""
    if buffer:
        normalized.append(buffer)
    return normalized


def parse_style_map(raw: str) -> dict[str, str]:
    """Parse Mermaid style fragments into a simple dictionary."""

    styles: dict[str, str] = {}
    for item in raw.split(","):
        item = item.strip()
        if not item or ":" not in item:
            continue
        key, value = item.split(":", 1)
        styles[key.strip()] = value.strip()
    return styles


def split_refs(segment: str) -> list[str]:
    """Split Mermaid multi-node edge references."""

    return [part.strip() for part in segment.split("&") if part.strip()]


def clean_label(text: str) -> str:
    """Normalize Mermaid label text."""

    return text.strip().strip('"').replace("<br/>", "\n").replace("<br>", "\n")


def parse_node_ref(token: str, nodes: dict[str, Node], current_group: Group, attach_existing: bool = False) -> str:
    """Create a node if the Mermaid token defines one and return its id."""

    token = token.strip()
    for pattern, shape in (
        (r"^(?P<node>[A-Za-z0-9_]+)\[\(\"(?P<label>.*)\"\)\]$", "cylinder"),
        (r"^(?P<node>[A-Za-z0-9_]+)\[\((?P<label>.*)\)\]$", "cylinder"),
        (r"^(?P<node>[A-Za-z0-9_]+)\[\"(?P<label>.*)\"\]$", "rect"),
        (r"^(?P<node>[A-Za-z0-9_]+)\[(?P<label>.*)\]$", "rect"),
    ):
        match = re.match(pattern, token)
        if match:
            node_id = match.group("node")
            label = clean_label(match.group("label"))
            is_new = node_id not in nodes
            node = nodes.setdefault(node_id, Node(node_id=node_id, label=label, shape=shape))
            node.label = label
            node.shape = shape
            if (is_new or attach_existing) and ("node", node_id) not in current_group.items:
                current_group.items.append(("node", node_id))
            return node_id

    match = NODE_ID_RE.match(token)
    if not match:
        raise ValueError(f"Unable to parse Mermaid node token: {token}")
    node_id = match.group(0)
    is_new = node_id not in nodes
    if node_id not in nodes:
        nodes[node_id] = Node(node_id=node_id, label=node_id)
    if (is_new or attach_existing) and ("node", node_id) not in current_group.items:
        current_group.items.append(("node", node_id))
    return node_id


def parse_ref(token: str, nodes: dict[str, Node], groups: dict[str, Group], current_group: Group) -> str:
    """Resolve a Mermaid edge endpoint to an existing group or a node."""

    token = token.strip()
    if token in groups:
        return token
    return parse_node_ref(token, nodes, current_group)


def parse_edge(line: str, nodes: dict[str, Node], groups: dict[str, Group], current_group: Group) -> list[Edge]:
    """Parse one Mermaid edge line into one or more connectors."""

    line = line.strip()
    animated = re.match(r"^(\S+)\s+\S+@--\s+(.+?)\s+-->\s+(.+)$", line)
    if animated:
        src = parse_ref(animated.group(1), nodes, groups, current_group)
        dst = parse_ref(animated.group(3), nodes, groups, current_group)
        return [Edge(source=src, target=dst, label=clean_label(animated.group(2)))]

    src_segment = ""
    dst_segment = ""
    op = ""
    label = ""
    if "|" in line:
        prefix, raw_label, suffix = line.split("|", 2)
        label = clean_label(raw_label)
        match = re.match(r"^(?P<src>.+?)\s*(?P<op><-->|<-->|<-->|<-->|-\.->|---|-->)\s*$", prefix.strip())
        if not match:
            raise ValueError(f"Unable to parse labeled Mermaid edge: {line}")
        src_segment = match.group("src")
        op = match.group("op")
        dst_segment = suffix.strip()
    else:
        match = re.match(r"^(?P<src>.+?)\s*(?P<op><-->|<-->|<-->|<-->|-\.->|---|-->)\s*(?P<dst>.+)$", line)
        if not match:
            return []
        src_segment = match.group("src")
        op = match.group("op")
        dst_segment = match.group("dst")

    sources = [parse_ref(part, nodes, groups, current_group) for part in split_refs(src_segment)]
    targets = [parse_ref(part, nodes, groups, current_group) for part in split_refs(dst_segment)]
    dashed = "." in op
    bidirectional = op.startswith("<") and op.endswith(">")
    undirected = op == "---"

    edges: list[Edge] = []
    for source in sources:
        for target in targets:
            edges.append(
                Edge(
                    source=source,
                    target=target,
                    label=label,
                    dashed=dashed,
                    bidirectional=bidirectional,
                    undirected=undirected,
                )
            )
    return edges


def parse_flowchart(text: str) -> FlowDiagram:
    """Parse the Mermaid flowchart subset used in this repo."""

    lines = normalize_flow_lines(text)
    first = lines[0].split()
    direction = first[1] if len(first) > 1 else "TB"

    root = Group(group_id="root", label="root", direction=direction)
    groups = {root.group_id: root}
    nodes: dict[str, Node] = {}
    edges: list[Edge] = []
    class_defs: dict[str, dict[str, str]] = {}
    stack = [root]

    for raw_line in lines[1:]:
        line = raw_line.strip()
        current_group = stack[-1]
        if line == "end":
            if len(stack) > 1:
                stack.pop()
            continue
        if line.startswith("subgraph "):
            match = re.match(r'^subgraph\s+(\w+)\["(.+)"\]$', line)
            if not match:
                match = re.match(r"^subgraph\s+(\w+)\[(.+)\]$", line)
            if not match:
                raise ValueError(f"Unable to parse subgraph line: {line}")
            group_id = match.group(1)
            label = clean_label(match.group(2))
            group = groups.setdefault(group_id, Group(group_id=group_id, label=label))
            group.label = label
            current_group.items.append(("group", group_id))
            stack.append(group)
            continue
        if line.startswith("direction "):
            current_group.direction = line.split()[1]
            continue
        if line.startswith("classDef "):
            _, class_name, style_raw = line.split(None, 2)
            class_defs[class_name] = parse_style_map(style_raw)
            continue
        if line.startswith("class "):
            match = re.match(r"^class\s+(.+?)\s+(\w+)$", line)
            if not match:
                continue
            ids = [item.strip() for item in match.group(1).split(",") if item.strip()]
            style_name = match.group(2)
            for item_id in ids:
                if item_id in nodes:
                    nodes[item_id].classes.append(style_name)
            continue
        if line.startswith("style "):
            match = re.match(r"^style\s+(\w+)\s+(.+)$", line)
            if not match:
                continue
            item_id, style_raw = match.groups()
            style = parse_style_map(style_raw)
            if item_id in nodes:
                nodes[item_id].style.update(style)
            elif item_id in groups:
                groups[item_id].style.update(style)
            continue
        if re.match(r"^\w+:::\w+$", line):
            item_id, style_name = line.split(":::", 1)
            if item_id in nodes:
                nodes[item_id].classes.append(style_name)
            continue
        if re.match(r"^\w+@\{.+\}$", line):
            continue

        parsed_edges = parse_edge(line, nodes, groups, current_group)
        if parsed_edges:
            edges.extend(parsed_edges)
            continue

        parse_node_ref(line, nodes, current_group, attach_existing=True)

    for node in nodes.values():
        merged_style: dict[str, str] = {}
        for class_name in node.classes:
            merged_style.update(class_defs.get(class_name, {}))
        merged_style.update(node.style)
        node.style = merged_style

    return FlowDiagram(direction=direction, nodes=nodes, groups=groups, edges=edges, root_group=root.group_id)


def parse_sequence(text: str) -> SequenceDiagram:
    """Parse the sequence diagram used in this repo."""

    participants: list[SequenceParticipant] = []
    events: list[SequenceEvent] = []
    blocks: list[SequenceBlock] = []
    stack: list[tuple[str, str]] = []

    for raw_line in strip_frontmatter(text).splitlines():
        line = raw_line.strip()
        if not line or line == "sequenceDiagram":
            continue
        participant_match = PARTICIPANT_RE.match(line)
        if participant_match:
            participants.append(
                SequenceParticipant(alias=participant_match.group(1), label=clean_label(participant_match.group(2)))
            )
            continue
        if line.startswith("rect "):
            block = SequenceBlock(block_id=f"block_{len(blocks) + 1}", color=line.split(None, 1)[1])
            blocks.append(block)
            stack.append(("rect", block.block_id))
            continue
        if line.startswith("par "):
            events.append(SequenceEvent(event_type="note", label=clean_label(line[4:]), span_start=None, span_end=None))
            stack.append(("par", f"par_{len(stack) + 1}"))
            continue
        if line == "end":
            if stack:
                stack.pop()
            continue
        note_match = NOTE_RE.match(line)
        if note_match:
            block_id = next((item_id for kind, item_id in reversed(stack) if kind == "rect"), None)
            events.append(
                SequenceEvent(
                    event_type="note",
                    label=clean_label(note_match.group(3)),
                    span_start=note_match.group(1),
                    span_end=note_match.group(2) or note_match.group(1),
                    block_id=block_id,
                )
            )
            continue
        message_match = MESSAGE_RE.match(line)
        if message_match:
            op = message_match.group(2)
            block_id = next((item_id for kind, item_id in reversed(stack) if kind == "rect"), None)
            events.append(
                SequenceEvent(
                    event_type="message",
                    source=message_match.group(1),
                    target=message_match.group(3),
                    label=clean_label(message_match.group(4)),
                    dashed="--" in op,
                    block_id=block_id,
                )
            )
            continue

    return SequenceDiagram(participants=participants, events=events, blocks=blocks)


def measure_node(node: Node) -> tuple[int, int]:
    """Estimate node size from text content."""

    lines = node.label.split("\n")
    width = min(max(max(len(line) for line in lines) * 7 + 36, 120), 280)
    height = max(len(lines) * 18 + 26, 54)
    node.width = width
    node.height = height
    return width, height


def measure_group(group: Group, groups: dict[str, Group], nodes: dict[str, Node]) -> tuple[int, int]:
    """Recursively measure a container and assign child relative positions."""

    padding = 24
    header = 34 if group.group_id != "root" else 0
    gap = 28
    sizes: list[tuple[str, str, int, int]] = []
    for kind, item_id in group.items:
        if kind == "node":
            width, height = measure_node(nodes[item_id])
        else:
            width, height = measure_group(groups[item_id], groups, nodes)
        sizes.append((kind, item_id, width, height))

    cursor = padding
    max_cross = 0
    for kind, item_id, width, height in sizes:
        if group.direction in {"LR", "RL"}:
            group.rel_positions[item_id] = (cursor, padding + header)
            cursor += width + gap
            max_cross = max(max_cross, height)
        else:
            group.rel_positions[item_id] = (padding, cursor + header)
            cursor += height + gap
            max_cross = max(max_cross, width)

    if not sizes:
        inner_width = 220
        inner_height = 80
    elif group.direction in {"LR", "RL"}:
        inner_width = cursor - gap + padding
        inner_height = max_cross + padding * 2 + header
    else:
        inner_width = max_cross + padding * 2
        inner_height = cursor - gap + padding + header

    group.width = inner_width
    group.height = inner_height
    return group.width, group.height


def place_group(group: Group, groups: dict[str, Group], nodes: dict[str, Node], origin_x: int, origin_y: int) -> None:
    """Recursively assign absolute positions."""

    group.x = origin_x
    group.y = origin_y
    for kind, item_id in group.items:
        rel_x, rel_y = group.rel_positions[item_id]
        if kind == "node":
            node = nodes[item_id]
            node.x = origin_x + rel_x
            node.y = origin_y + rel_y
        else:
            place_group(groups[item_id], groups, nodes, origin_x + rel_x, origin_y + rel_y)


def style_to_drawio(style: dict[str, str], shape: str) -> str:
    """Map Mermaid styles to draw.io style strings."""

    fill = style.get("fill", "#ffffff")
    stroke = style.get("stroke", "#1f2933")
    font = style.get("color", "#1f2933")
    if shape == "cylinder":
        base = "shape=cylinder3;boundedLbl=1;backgroundOutline=1;size=15;"
    else:
        base = "rounded=1;"
    return (
        f"{base}whiteSpace=wrap;html=1;fillColor={fill};strokeColor={stroke};"
        f"fontColor={font};align=center;verticalAlign=middle;"
    )


def group_style_to_drawio(style: dict[str, str]) -> str:
    """Map Mermaid subgraph styles to draw.io swimlane styling."""

    fill = style.get("fill", "#f8fafc")
    stroke = style.get("stroke", "#94a3b8")
    return (
        "swimlane;fontStyle=1;html=1;whiteSpace=wrap;rounded=0;"
        f"fillColor={fill};strokeColor={stroke};startSize=28;"
    )


def edge_style_to_drawio(edge: Edge) -> str:
    """Map Mermaid connectors to draw.io connector styles."""

    end_arrow = "none" if edge.undirected else "blockThin"
    start_arrow = "blockThin" if edge.bidirectional else "none"
    dashed = "1" if edge.dashed else "0"
    return (
        "edgeStyle=orthogonalEdgeStyle;rounded=0;orthogonalLoop=1;jettySize=auto;html=1;"
        f"dashed={dashed};endArrow={end_arrow};startArrow={start_arrow};strokeColor=#6b7280;"
    )


def add_mxcell(root: ET.Element, cell_id: str, value: str, style: str, vertex: bool, parent: str, x: int, y: int, width: int, height: int) -> None:
    """Add a draw.io vertex cell."""

    cell = ET.SubElement(
        root,
        "mxCell",
        attrib={
            "id": cell_id,
            "value": value,
            "style": style,
            "parent": parent,
            "vertex": "1" if vertex else "0",
        },
    )
    ET.SubElement(
        cell,
        "mxGeometry",
        attrib={
            "x": str(x),
            "y": str(y),
            "width": str(width),
            "height": str(height),
            "as": "geometry",
        },
    )


def add_edge_cell(root: ET.Element, cell_id: str, edge: Edge, source_cell: str, target_cell: str) -> None:
    """Add a draw.io connector cell."""

    cell = ET.SubElement(
        root,
        "mxCell",
        attrib={
            "id": cell_id,
            "value": html.escape(edge.label).replace("\n", "<br>"),
            "style": edge_style_to_drawio(edge),
            "parent": "1",
            "edge": "1",
            "source": source_cell,
            "target": target_cell,
        },
    )
    ET.SubElement(cell, "mxGeometry", attrib={"relative": "1", "as": "geometry"})


def build_flow_drawio(diagram: FlowDiagram, page_name: str) -> ET.ElementTree:
    """Render a parsed flowchart into draw.io XML."""

    measure_group(diagram.groups[diagram.root_group], diagram.groups, diagram.nodes)
    place_group(diagram.groups[diagram.root_group], diagram.groups, diagram.nodes, 20, 20)

    mxfile = ET.Element(
        "mxfile",
        host="app.diagrams.net",
        modified=dt.datetime.now(dt.UTC).strftime("%Y-%m-%dT%H:%M:%SZ"),
        agent="GitHub Copilot",
        version="24.7.17",
    )
    diagram_el = ET.SubElement(mxfile, "diagram", id=uuid.uuid4().hex[:12], name=page_name)
    graph_model = ET.SubElement(
        diagram_el,
        "mxGraphModel",
        dx="1360",
        dy="900",
        grid="1",
        gridSize="10",
        guides="1",
        tooltips="1",
        connect="1",
        arrows="1",
        fold="1",
        page="1",
        pageScale="1",
        pageWidth="1800",
        pageHeight="1200",
        math="0",
        shadow="0",
    )
    root = ET.SubElement(graph_model, "root")
    ET.SubElement(root, "mxCell", id="0")
    ET.SubElement(root, "mxCell", attrib={"id": "1", "parent": "0"})

    cell_ids: dict[str, str] = {}
    index = 2
    for group_id, group in diagram.groups.items():
        if group_id == diagram.root_group:
            continue
        cell_id = f"cell_{index}"
        index += 1
        cell_ids[group_id] = cell_id
        add_mxcell(
            root,
            cell_id,
            html.escape(group.label).replace("\n", "<br>"),
            group_style_to_drawio(group.style),
            True,
            "1",
            group.x,
            group.y,
            group.width,
            group.height,
        )

    for node_id, node in diagram.nodes.items():
        cell_id = f"cell_{index}"
        index += 1
        cell_ids[node_id] = cell_id
        add_mxcell(
            root,
            cell_id,
            html.escape(node.label).replace("\n", "<br>"),
            style_to_drawio(node.style, node.shape),
            True,
            "1",
            node.x,
            node.y,
            node.width,
            node.height,
        )

    for edge in diagram.edges:
        cell_id = f"cell_{index}"
        index += 1
        add_edge_cell(root, cell_id, edge, cell_ids[edge.source], cell_ids[edge.target])

    return ET.ElementTree(mxfile)


def build_sequence_drawio(diagram: SequenceDiagram, page_name: str) -> ET.ElementTree:
    """Render the sequence diagram into draw.io XML."""

    x_margin = 50
    y_margin = 40
    lane_width = 180
    header_width = 140
    header_height = 54
    current_y = 120
    block_ranges: dict[str, list[int]] = {}

    for event in diagram.events:
        if event.event_type == "note":
            event.height = 60
        else:
            event.height = 48
        event.y = current_y
        current_y += event.height + 22
        if event.block_id:
            block_ranges.setdefault(event.block_id, [event.y, event.y + event.height])
            block_ranges[event.block_id][1] = event.y + event.height

    for block in diagram.blocks:
        start_y, end_y = block_ranges.get(block.block_id, [current_y, current_y + 40])
        block.start_y = start_y - 20
        block.end_y = end_y + 20

    participant_x = {participant.alias: x_margin + index * lane_width for index, participant in enumerate(diagram.participants)}
    total_width = x_margin * 2 + lane_width * max(len(diagram.participants) - 1, 0) + header_width
    total_height = current_y + 80

    mxfile = ET.Element(
        "mxfile",
        host="app.diagrams.net",
        modified=dt.datetime.now(dt.UTC).strftime("%Y-%m-%dT%H:%M:%SZ"),
        agent="GitHub Copilot",
        version="24.7.17",
    )
    diagram_el = ET.SubElement(mxfile, "diagram", id=uuid.uuid4().hex[:12], name=page_name)
    graph_model = ET.SubElement(
        diagram_el,
        "mxGraphModel",
        dx="1360",
        dy="900",
        grid="1",
        gridSize="10",
        guides="1",
        tooltips="1",
        connect="1",
        arrows="1",
        fold="1",
        page="1",
        pageScale="1",
        pageWidth="1800",
        pageHeight="1400",
        math="0",
        shadow="0",
    )
    root = ET.SubElement(graph_model, "root")
    ET.SubElement(root, "mxCell", id="0")
    ET.SubElement(root, "mxCell", attrib={"id": "1", "parent": "0"})

    index = 2
    participant_cells: dict[str, str] = {}
    for block in diagram.blocks:
        cell = ET.SubElement(
            root,
            "mxCell",
            attrib={
                "id": f"cell_{index}",
                "value": "",
                "style": f"rounded=0;whiteSpace=wrap;html=1;fillColor={block.color};strokeColor=#cbd5e1;opacity=55;",
                "parent": "1",
                "vertex": "1",
            },
        )
        ET.SubElement(
            cell,
            "mxGeometry",
            attrib={
                "x": str(x_margin - 20),
                "y": str(block.start_y),
                "width": str(total_width - x_margin + 10),
                "height": str(block.end_y - block.start_y),
                "as": "geometry",
            },
        )
        index += 1

    for participant in diagram.participants:
        x = participant_x[participant.alias]
        header_cell = ET.SubElement(
            root,
            "mxCell",
            attrib={
                "id": f"cell_{index}",
                "value": html.escape(participant.label).replace("\n", "<br>"),
                "style": "rounded=1;whiteSpace=wrap;html=1;fillColor=#dae8fc;strokeColor=#6c8ebf;fontColor=#1f2933;",
                "parent": "1",
                "vertex": "1",
            },
        )
        ET.SubElement(
            header_cell,
            "mxGeometry",
            attrib={
                "x": str(x),
                "y": str(y_margin),
                "width": str(header_width),
                "height": str(header_height),
                "as": "geometry",
            },
        )
        participant_cells[participant.alias] = f"cell_{index}"
        index += 1

        line_cell = ET.SubElement(
            root,
            "mxCell",
            attrib={
                "id": f"cell_{index}",
                "value": "",
                "style": "shape=line;strokeColor=#94a3b8;dashed=1;direction=south;html=1;",
                "parent": "1",
                "vertex": "1",
            },
        )
        ET.SubElement(
            line_cell,
            "mxGeometry",
            attrib={
                "x": str(x + header_width // 2),
                "y": str(y_margin + header_height),
                "width": "1",
                "height": str(total_height - y_margin - header_height),
                "as": "geometry",
            },
        )
        index += 1

    for event in diagram.events:
        if event.event_type == "note":
            if event.span_start and event.span_end:
                start_x = participant_x.get(event.span_start, x_margin)
                end_x = participant_x.get(event.span_end, start_x)
                width = abs(end_x - start_x) + header_width
                x = min(start_x, end_x)
            else:
                x = x_margin + 80
                width = total_width - 160
            note_cell = ET.SubElement(
                root,
                "mxCell",
                attrib={
                    "id": f"cell_{index}",
                    "value": html.escape(event.label).replace("\n", "<br>"),
                    "style": "rounded=0;whiteSpace=wrap;html=1;fillColor=#fff2cc;strokeColor=#d6b656;fontColor=#1f2933;",
                    "parent": "1",
                    "vertex": "1",
                },
            )
            ET.SubElement(
                note_cell,
                "mxGeometry",
                attrib={
                    "x": str(x),
                    "y": str(event.y),
                    "width": str(width),
                    "height": str(event.height),
                    "as": "geometry",
                },
            )
            index += 1
            continue

        if event.source is None or event.target is None:
            continue
        source_x = participant_x[event.source] + header_width // 2
        target_x = participant_x[event.target] + header_width // 2
        label_width = max(abs(target_x - source_x), 220)
        edge_cell = ET.SubElement(
            root,
            "mxCell",
            attrib={
                "id": f"cell_{index}",
                "value": html.escape(event.label).replace("\n", "<br>"),
                "style": (
                    "edgeStyle=orthogonalEdgeStyle;rounded=0;html=1;"
                    f"dashed={'1' if event.dashed else '0'};endArrow=blockThin;strokeColor=#4b5563;"
                ),
                "parent": "1",
                "edge": "1",
            },
        )
        geom = ET.SubElement(edge_cell, "mxGeometry", attrib={"relative": "1", "as": "geometry"})
        ET.SubElement(geom, "mxPoint", attrib={"x": str(source_x), "y": str(event.y + 18), "as": "sourcePoint"})
        ET.SubElement(geom, "mxPoint", attrib={"x": str(target_x), "y": str(event.y + 18), "as": "targetPoint"})
        index += 1

        label_cell = ET.SubElement(
            root,
            "mxCell",
            attrib={
                "id": f"cell_{index}",
                "value": html.escape(event.label).replace("\n", "<br>"),
                "style": "text;html=1;strokeColor=none;fillColor=none;align=center;verticalAlign=middle;whiteSpace=wrap;",
                "parent": "1",
                "vertex": "1",
            },
        )
        ET.SubElement(
            label_cell,
            "mxGeometry",
            attrib={
                "x": str(min(source_x, target_x) + 10),
                "y": str(event.y - 2),
                "width": str(label_width - 20),
                "height": "22",
                "as": "geometry",
            },
        )
        index += 1

    page = ET.SubElement(
        root,
        "mxCell",
        attrib={
            "id": f"cell_{index}",
            "value": "",
            "style": "rounded=0;strokeColor=none;fillColor=none;",
            "parent": "1",
            "vertex": "1",
        },
    )
    ET.SubElement(
        page,
        "mxGeometry",
        attrib={
            "x": "0",
            "y": "0",
            "width": str(total_width),
            "height": str(total_height),
            "as": "geometry",
        },
    )

    return ET.ElementTree(mxfile)


def indent_xml(tree: ET.ElementTree) -> None:
    """Format XML output for readability."""

    ET.indent(tree, space="  ")


def convert_file(source: Path, destination: Path) -> None:
    """Convert one Mermaid file into a draw.io file."""

    text = source.read_text(encoding="utf-8")
    stripped = strip_frontmatter(text).lstrip()
    if stripped.startswith("sequenceDiagram"):
        tree = build_sequence_drawio(parse_sequence(text), source.stem)
    else:
        tree = build_flow_drawio(parse_flowchart(text), source.stem)
    indent_xml(tree)
    tree.write(destination, encoding="utf-8", xml_declaration=True)


def iter_sources(path: Path) -> list[Path]:
    """Return Mermaid files under a file or directory input."""

    if path.is_file():
        return [path]
    return sorted(path.rglob("*.mmd"))


def main() -> None:
    """CLI entry point."""

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", type=Path, help="Mermaid file or directory to convert")
    args = parser.parse_args()

    for source in iter_sources(args.path):
        destination = source.with_suffix(".drawio")
        convert_file(source, destination)
        print(f"wrote {destination}")


if __name__ == "__main__":
    main()