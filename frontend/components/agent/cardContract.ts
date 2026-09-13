export type CardPreset = {
  label: string;
  value: string;
};

export type CardStep = {
  n: string;
  call: string;
  detail: string;
};

export type CardButtonLabels = {
  ready: string;
  arming: string;
  approving: string;
  submitting_approve?: string;
  executing: string;
  executed: string;
  connect_wallet: string;
  enter_budget: string;
  acknowledge_required: string;
};

export type CardFootnotes = {
  signatures_needed: string;
  executed_success: string;
  execution_failed: string;
};

export type CardNeedsInput = {
  notice: string;
  placeholder: string;
  button: string;
};

export type CardLedger = {
  armed_title: string;
  executed_title: string;
  paused_title: string;
  paused_desc: string;
  disarmed_title: string;
  disarmed_desc: string;
  executed_desc: string;
  toggle_show: string;
  toggle_hide: string;
  trades_header: string;
  no_trades_yet: string;
  multi_trade_notice: string;
  disarm_notice: string;
  resume_button: string;
  pause_button: string;
  disarm_button: string;
};

export type CardContract = {
  task_badge: string;
  subtask_unit: string;
  header_description: string;
  genesis_label: string;
  arm_title_ready: string;
  arm_title_armed: string;
  arm_description: string;
  no_trade_description: string;
  budget_label: string;
  budget_placeholder: string;
  preset_label: string;
  presets: CardPreset[];
  steps: CardStep[];
  disclaimer: string;
  button_labels: CardButtonLabels;
  footnotes: CardFootnotes;
  needs_input: CardNeedsInput;
  status_labels: Record<string, string>;
  ledger: CardLedger;
};

const DEFAULT_CARD_CONTRACT: CardContract = {
  task_badge: 'Task T-{id}',
  subtask_unit: 'sub task',
  header_description:
    'This entire card is one Task. Each numbered row is a Sub Task: a single step executed by an agent with its own reasoning and verification hash. Expand a row to inspect output details.',
  genesis_label: 'genesis {hash} · keccak256(task_id, trigger, owner)',
  arm_title_ready: 'Confirm and Arm Transaction',
  arm_title_armed: 'Transaction Ready & Armed',
  arm_description:
    'Your IDRX tokens never leave your wallet. You are granting an allowance limit that the smart contract pulls upon execution, strictly capped, and revokable at any time.',
  no_trade_description:
    'This Task does not contain an on-chain trade — arming only records your request and reasoning proofs on-chain.',
  budget_label: 'Maximum Budget Cap (IDRX)',
  budget_placeholder: 'e.g. 1,000,000',
  preset_label: 'Quick Presets:',
  presets: [
    { label: '100 Rb', value: '100000' },
    { label: '500 Rb', value: '500000' },
    { label: '1 Jt', value: '1000000' },
    { label: '5 Jt', value: '5000000' },
    { label: '10 Jt', value: '10000000' },
    { label: '20 Jt', value: '20000000' },
  ],
  steps: [
    { n: '01', call: 'createTask + grantTradePermission', detail: 'Records Task hash commitment on-chain, signed by Quasar operator.' },
    { n: '02', call: 'approve(IDRX)', detail: 'Your wallet approves the allowance limit pulled by the contract upon execution.' },
    { n: '03', call: 'comet.executeTrade', detail: 'Comet analyzes live spot price and executes swap on Uniswap V4.' },
  ],
  disclaimer:
    'I understand this is the only confirmation requested. Once armed, Comet acts within this limit autonomously until I cancel this Task — and I retain full control at all times.',
  button_labels: {
    ready: 'Arm & Execute Transaction',
    arming: '1/3: Recording Task permission on-chain…',
    approving: '2/3: Awaiting allowance signature in MetaMask…',
    submitting_approve: '2/3: On-chain submitted. Waiting for block confirmation…',
    executing: '3/3: On-chain confirmed. Processing execution on server (Comet)…',
    executed: 'Transaction Successfully Executed',
    connect_wallet: 'Connect Wallet First',
    enter_budget: 'Enter a Valid IDRX Budget',
    acknowledge_required: 'Check confirmation above to proceed',
  },
  footnotes: {
    signatures_needed:
      'Signatures: on-chain Task permission, then ERC20 allowance. Afterwards, Comet immediately executes on Uniswap V4.',
    executed_success: 'Transaction successfully executed on Uniswap V4!',
    execution_failed: 'Order could not be executed on-chain. Comet sent the details to the chat message above.',
  },
  needs_input: {
    notice:
      'This Sub Task has status needs_input. Parameters cannot be guessed — execution cannot be armed while required information is missing.',
    placeholder: 'Your answer',
    button: 'Send answer',
  },
  status_labels: {
    done: 'DONE',
    completed: 'DONE',
    needs_input: 'NEEDS INPUT',
    running: 'RUNNING',
    failed: 'FAILED',
    error: 'FAILED',
  },
  ledger: {
    armed_title: 'Armed · Task T-{id} · on-chain #{onChainTaskId}',
    executed_title: 'Executed · Task T-{id} · on-chain #{onChainTaskId}',
    paused_title: 'Loop Paused by You',
    paused_desc:
      'Evaluation temporarily paused. Market data is still tracked, but no decisions are processed until you resume. This Task keeps its signature — this is a temporary pause, not a cancellation.',
    disarmed_title: 'Disarmed · Task T-{id}',
    disarmed_desc:
      'On-chain Task has been cancelled and allowance reset to zero. Agent autonomy ends with your signature — no funds are held beyond already settled executions.',
    executed_desc:
      'Transaction has finished executing on-chain. To open or close a new position, please start a new Task via chat.',
    toggle_show: 'show',
    toggle_hide: 'hide',
    trades_header: 'On-chain Task #{onChainTaskId} · {count} trade(s)',
    no_trades_yet: 'No trades yet.',
    multi_trade_notice:
      'One on-chain Task ID can shelter multiple trades. #{onChainTaskId} remains identical for the life of the Task; each fill emits its own TradeExecuted event with its own transaction hash.',
    disarm_notice:
      'Disarm cancels on-chain Task #{onChainTaskId} and resets allowance to zero. This is your personal safety switch.',
    resume_button: 'Resume',
    pause_button: 'Pause',
    disarm_button: 'Disarm',
  },
};

export function parseCardContract(task?: { trigger_description?: string | null } | null): CardContract {
  if (!task?.trigger_description) {
    return DEFAULT_CARD_CONTRACT;
  }

  try {
    const parsed = JSON.parse(task.trigger_description);
    if (!parsed || typeof parsed !== 'object') {
      return DEFAULT_CARD_CONTRACT;
    }

    const c = parsed.card;
    if (!c || typeof c !== 'object') {
      return DEFAULT_CARD_CONTRACT;
    }

    return {
      task_badge: c.task_badge || DEFAULT_CARD_CONTRACT.task_badge,
      subtask_unit: c.subtask_unit || DEFAULT_CARD_CONTRACT.subtask_unit,
      header_description: c.header_description || DEFAULT_CARD_CONTRACT.header_description,
      genesis_label: c.genesis_label || DEFAULT_CARD_CONTRACT.genesis_label,
      arm_title_ready: c.arm_title_ready || DEFAULT_CARD_CONTRACT.arm_title_ready,
      arm_title_armed: c.arm_title_armed || DEFAULT_CARD_CONTRACT.arm_title_armed,
      arm_description: c.arm_description || DEFAULT_CARD_CONTRACT.arm_description,
      no_trade_description: c.no_trade_description || DEFAULT_CARD_CONTRACT.no_trade_description,
      budget_label: c.budget_label || DEFAULT_CARD_CONTRACT.budget_label,
      budget_placeholder: c.budget_placeholder || DEFAULT_CARD_CONTRACT.budget_placeholder,
      preset_label: c.preset_label || DEFAULT_CARD_CONTRACT.preset_label,
      presets: Array.isArray(c.presets) && c.presets.length > 0 ? c.presets : DEFAULT_CARD_CONTRACT.presets,
      steps: Array.isArray(c.steps) && c.steps.length > 0 ? c.steps : DEFAULT_CARD_CONTRACT.steps,
      disclaimer: c.disclaimer || DEFAULT_CARD_CONTRACT.disclaimer,
      button_labels: {
        ...DEFAULT_CARD_CONTRACT.button_labels,
        ...(c.button_labels || {}),
      },
      footnotes: {
        ...DEFAULT_CARD_CONTRACT.footnotes,
        ...(c.footnotes || {}),
      },
      needs_input: {
        ...DEFAULT_CARD_CONTRACT.needs_input,
        ...(c.needs_input || {}),
      },
      status_labels: {
        ...DEFAULT_CARD_CONTRACT.status_labels,
        ...(c.status_labels || {}),
      },
      ledger: {
        ...DEFAULT_CARD_CONTRACT.ledger,
        ...(c.ledger || {}),
      },
    };
  } catch {
    return DEFAULT_CARD_CONTRACT;
  }
}
