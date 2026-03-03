#!/usr/bin/env python3
"""
Post-processor: inject drag-and-drop JavaScript into mermaid SVGs.
Nodes are draggable and connected edges + labels follow along.
"""

import glob
import os
import re
import sys

DRAG_SCRIPT = r'''
<script type="text/javascript"><![CDATA[
(function(){
  var svg = document.querySelector('svg');
  var pt = svg.createSVGPoint();
  var dragTarget = null;
  var startX, startY, origTx, origTy;
  var connectedEdges = [];  // [{path, label, end:'start'|'end'|'both', origD, origLabelTr}]

  function getTranslate(el) {
    var m = /translate\(([\d.e+-]+)[,\s]+([\d.e+-]+)\)/.exec(el.getAttribute('transform') || '');
    return m ? [parseFloat(m[1]), parseFloat(m[2])] : null;
  }

  function setTranslate(el, x, y) {
    var t = el.getAttribute('transform') || '';
    var nt = 'translate(' + x + ', ' + y + ')';
    if (/translate\(/.test(t)) {
      el.setAttribute('transform', t.replace(/translate\([^)]+\)/, nt));
    } else {
      el.setAttribute('transform', nt + ' ' + t);
    }
  }

  function toSVG(evt) {
    pt.x = evt.clientX; pt.y = evt.clientY;
    return pt.matrixTransform(svg.getScreenCTM().inverse());
  }

  function findDraggable(el) {
    while (el && el !== svg) {
      if (el.classList && el.classList.contains('node')) return el;
      el = el.parentNode;
    }
    return null;
  }

  // Build node ID -> mermaid short name mapping
  // Node id like "flowchart-APRS-0" -> short name "APRS"
  function getNodeShortId(node) {
    var id = node.id || '';
    var m = /^flowchart-(.+)-\d+$/.exec(id);
    return m ? m[1] : id;
  }

  // Parse SVG path d attribute into segments for manipulation
  function parsePath(d) {
    var segs = [];
    var re = /([MLCQTSAHVZ])\s*((?:[^MLCQTSAHVZ])*)/gi;
    var m;
    while ((m = re.exec(d)) !== null) {
      var cmd = m[1];
      var nums = m[2].trim().match(/-?[\d.]+(?:e[+-]?\d+)?/gi);
      segs.push({ cmd: cmd, nums: nums ? nums.map(Number) : [] });
    }
    return segs;
  }

  function segsToD(segs) {
    return segs.map(function(s) {
      return s.cmd + s.nums.join(',');
    }).join('');
  }

  // Shift the start portion of path segments by (dx, dy)
  function shiftPathStart(segs, dx, dy) {
    if (segs.length === 0) return;
    // First segment is M x,y
    if (segs[0].cmd === 'M' && segs[0].nums.length >= 2) {
      segs[0].nums[0] += dx;
      segs[0].nums[1] += dy;
    }
    // Shift first curve/line segment's first control point
    if (segs.length > 1) {
      var s = segs[1];
      if (s.cmd === 'L' && s.nums.length >= 2) {
        s.nums[0] += dx; s.nums[1] += dy;
      } else if (s.cmd === 'C' && s.nums.length >= 6) {
        // first control point
        s.nums[0] += dx; s.nums[1] += dy;
      }
    }
  }

  // Shift the end portion of path segments by (dx, dy)
  function shiftPathEnd(segs, dx, dy) {
    if (segs.length === 0) return;
    var last = segs[segs.length - 1];
    if (last.cmd === 'L' && last.nums.length >= 2) {
      last.nums[last.nums.length - 2] += dx;
      last.nums[last.nums.length - 1] += dy;
    } else if (last.cmd === 'C' && last.nums.length >= 6) {
      // last control point and end point
      last.nums[last.nums.length - 4] += dx;
      last.nums[last.nums.length - 3] += dy;
      last.nums[last.nums.length - 2] += dx;
      last.nums[last.nums.length - 1] += dy;
    }
    // Also shift the previous segment's endpoint if it feeds into the last
    if (segs.length > 1) {
      var prev = segs[segs.length - 2];
      if (prev.cmd === 'C' && prev.nums.length >= 6) {
        prev.nums[prev.nums.length - 2] += dx;
        prev.nums[prev.nums.length - 1] += dy;
      }
    }
  }

  // Find all edges and labels connected to a node
  function findEdges(nodeId) {
    var results = [];
    var paths = svg.querySelectorAll('path[data-id]');
    for (var i = 0; i < paths.length; i++) {
      var p = paths[i];
      var eid = p.getAttribute('data-id') || '';
      // Edge ID format: L_SOURCE_TARGET_N
      // But node names can contain underscores, so we use a smarter match
      var parts = parseEdgeId(eid);
      if (!parts) continue;
      var end = null;
      if (parts.src === nodeId && parts.dst === nodeId) end = 'both';
      else if (parts.src === nodeId) end = 'start';
      else if (parts.dst === nodeId) end = 'end';
      if (!end) continue;

      // Find matching edge label
      var labelEl = svg.querySelector('.edgeLabel g.label[data-id="' + eid + '"]');
      var labelParent = labelEl ? labelEl.closest('.edgeLabel') : null;

      results.push({
        path: p,
        end: end,
        origD: p.getAttribute('d'),
        label: labelParent,
        origLabelTr: labelParent ? getTranslate(labelParent) : null
      });
    }
    return results;
  }

  // Parse edge ID like "L_APRS_APRSTV_0" -> {src: "APRS", dst: "APRSTV"}
  // Handle multi-underscore node names by matching against known node IDs
  var allNodeIds = [];
  function buildNodeList() {
    var nodes = svg.querySelectorAll('.node');
    for (var i = 0; i < nodes.length; i++) {
      allNodeIds.push(getNodeShortId(nodes[i]));
    }
    // Sort by length descending for greedy matching
    allNodeIds.sort(function(a, b) { return b.length - a.length; });
  }

  function parseEdgeId(eid) {
    // Format: L_<src>_<dst>_<index>
    if (eid.charAt(0) !== 'L' || eid.charAt(1) !== '_') return null;
    var body = eid.substring(2); // remove "L_"
    // Try to match known node IDs
    for (var i = 0; i < allNodeIds.length; i++) {
      var src = allNodeIds[i];
      if (body.indexOf(src + '_') === 0) {
        var rest = body.substring(src.length + 1);
        for (var j = 0; j < allNodeIds.length; j++) {
          var dst = allNodeIds[j];
          if (rest.indexOf(dst + '_') === 0) {
            return { src: src, dst: dst };
          }
        }
      }
    }
    return null;
  }

  buildNodeList();

  svg.addEventListener('mousedown', function(e) {
    var target = findDraggable(e.target);
    if (!target) return;
    var tr = getTranslate(target);
    if (!tr) return;
    dragTarget = target;
    origTx = tr[0]; origTy = tr[1];
    var p = toSVG(e);
    startX = p.x; startY = p.y;
    // Collect connected edges
    connectedEdges = findEdges(getNodeShortId(target));
    dragTarget.style.cursor = 'grabbing';
    e.preventDefault();
  });

  svg.addEventListener('mousemove', function(e) {
    if (!dragTarget) return;
    var p = toSVG(e);
    var dx = p.x - startX, dy = p.y - startY;

    // Move node
    setTranslate(dragTarget, origTx + dx, origTy + dy);

    // Update connected edges
    for (var i = 0; i < connectedEdges.length; i++) {
      var ce = connectedEdges[i];
      var segs = parsePath(ce.origD);
      if (ce.end === 'start' || ce.end === 'both') {
        shiftPathStart(segs, dx, dy);
      }
      if (ce.end === 'end' || ce.end === 'both') {
        shiftPathEnd(segs, dx, dy);
      }
      ce.path.setAttribute('d', segsToD(segs));

      // Move label proportionally
      if (ce.label && ce.origLabelTr) {
        var ratio = (ce.end === 'both') ? 1 : 0.5;
        setTranslate(ce.label,
          ce.origLabelTr[0] + dx * ratio,
          ce.origLabelTr[1] + dy * ratio);
      }
    }

    // Update marker arrow positions (re-rendered by browser automatically)
    e.preventDefault();
  });

  function endDrag() {
    if (dragTarget) {
      dragTarget.style.cursor = '';
      dragTarget = null;
      connectedEdges = [];
    }
  }
  svg.addEventListener('mouseup', endDrag);
  svg.addEventListener('mouseleave', endDrag);

  // Add grab cursor
  var allNodes = svg.querySelectorAll('.node');
  for (var i = 0; i < allNodes.length; i++) {
    allNodes[i].style.cursor = 'grab';
  }
})();
]]></script>
'''


def inject_drag(svg_content):
    """Inject drag script, replacing any existing script."""
    # Remove existing script if present
    cleaned = re.sub(r'<script[^>]*>.*?</script>\s*', '', svg_content, flags=re.DOTALL)
    return cleaned.replace('</svg>', DRAG_SCRIPT + '</svg>')


def process_file(filepath):
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()
    result = inject_drag(content)
    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(result)
    print(f'  + {os.path.basename(filepath)}')


def main():
    if len(sys.argv) > 1:
        files = sys.argv[1:]
    else:
        script_dir = os.path.dirname(os.path.abspath(__file__))
        files = glob.glob(os.path.join(script_dir, '*.svg'))
    if not files:
        print('No SVG files found')
        return
    print('Injecting drag script:')
    for f in sorted(files):
        process_file(f)
    print('Done.')


if __name__ == '__main__':
    main()
