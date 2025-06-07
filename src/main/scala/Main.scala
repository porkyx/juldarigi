import com.raquo.laminar.api.L.{*, given}
import org.scalajs.dom

import scala.scalajs.concurrent.JSExecutionContext.Implicits.queue
import scala.scalajs.js

@main
def JuldarigiApp(): Unit =
  renderOnDomContentLoaded(
    dom.document.getElementById("app"),
    Main.appElement()
  )

object Main {
  import ScraperService._

  // State management
  private val urlInput = Var("")
  private val scrapeMode = Var[ScrapeMode](PageMode(1))
  private val pagesInput = Var("1")
  private val startDateInput = Var("")
  private val endDateInput = Var("")

  private val isLoading = Var(false)
  private val progressInfo = Var[Option[ProgressInfo]](None)
  private val result = Var[Option[DCGalleryResult]](None)
  private val errorMessage = Var[Option[String]](None)

  // Clipboard functionality - simplified and more reliable
  private def copyTableData(userStats: List[UserStat]): Unit = {
    // 간단한 텍스트 형태 생성 (스프레드시트와 위지윅 에디터 모두 호환)
    val textTable = {
      val headerText = "순위\t닉네임\t식별코드\tIP\t게시물 수"
      val dataText = userStats.zipWithIndex
        .map { case (user, index) =>
          s"${index + 1}\t${user.nickname}\t${user.uid}\t${user.ip}\t${user.count}개"
        }
        .mkString("\n")
      s"$headerText\n$dataText"
    }

    // 최신 Clipboard API 사용
    if (dom.window.navigator.clipboard != null) {
      dom.window.navigator.clipboard
        .writeText(textTable)
        .toFuture
        .map { _ =>
          dom.window.alert("테이블 데이터가 클립보드에 복사되었습니다!\n(스프레드시트나 에디터에 붙여넣으세요)")
        }
        .recover { case _ =>
          copyTableFallback(textTable)
        }
    } else {
      // 구형 브라우저를 위한 fallback
      copyTableFallback(textTable)
    }
  }

  private def copyTableFallback(textTable: String): Unit = {
    try {
      val textArea =
        dom.document.createElement("textarea").asInstanceOf[dom.html.TextArea]
      textArea.value = textTable
      textArea.style.position = "fixed"
      textArea.style.left = "-999999px"
      textArea.style.top = "-999999px"
      textArea.style.opacity = "0"
      dom.document.body.appendChild(textArea)
      textArea.focus()
      textArea.select()

      val success = dom.document.execCommand("copy")
      dom.document.body.removeChild(textArea)

      if (success) {
        dom.window.alert("테이블 데이터가 클립보드에 복사되었습니다!")
      } else {
        dom.window.alert("복사에 실패했습니다. '전체선택' 버튼을 사용해보세요.")
      }
    } catch {
      case _: Exception =>
        dom.window.alert("복사에 실패했습니다. '전체선택' 버튼을 사용해보세요.")
    }
  }

  // 테이블 전체 선택 기능
  private def selectTableContent(): Unit = {
    try {
      // 테이블 요소를 찾아서 선택
      val tables = dom.document.querySelectorAll("table")
      if (tables.length > 0) {
        val table = tables.item(0)
        val range = dom.document.createRange()
        range.selectNodeContents(table)
        val selection = dom.window.getSelection()
        selection.removeAllRanges()
        selection.addRange(range)
        dom.window.alert("테이블이 선택되었습니다. Ctrl+C (또는 Cmd+C)로 복사하세요.")
      } else {
        dom.window.alert("테이블을 찾을 수 없습니다.")
      }
    } catch {
      case _: Exception =>
        dom.window.alert("테이블 선택에 실패했습니다.")
    }
  }

  // CSV 내보내기 기능 - 브라우저 기본 다운로드 방식
  private def exportToCSV(
      userStats: List[UserStat],
      galleryId: String
  ): Unit = {
    try {
      dom.console.log(
        s"Exporting CSV for gallery: $galleryId, users: ${userStats.length}"
      )

      // CSV 데이터 생성
      val csvHeader = "순위,닉네임,식별코드,IP,게시물수"
      val csvData = userStats.zipWithIndex
        .map { case (user, index) =>
          // CSV에서 특수문자 처리 (쉼표, 따옴표 등)
          val escapedNickname = escapeCSVField(user.nickname)
          val escapedUid = escapeCSVField(user.uid)
          val escapedIp = escapeCSVField(user.ip)
          s"${index + 1},$escapedNickname,$escapedUid,$escapedIp,${user.count}"
        }
        .mkString("\n")

      val csvContent = s"$csvHeader\n$csvData"

      // BOM 추가 (한글 깨짐 방지)
      val bom = "\uFEFF"
      val csvWithBom = bom + csvContent

      // 현재 날짜/시간을 파일명에 포함
      val now = new js.Date()
      val year = now.getFullYear().toInt
      val month =
        (now.getMonth().toInt + 1).toString.reverse.padTo(2, '0').reverse
      val day = now.getDate().toInt.toString.reverse.padTo(2, '0').reverse
      val hour = now.getHours().toInt.toString.reverse.padTo(2, '0').reverse
      val minute = now.getMinutes().toInt.toString.reverse.padTo(2, '0').reverse

      val fileName =
        s"갤창랭킹_${galleryId}_${year}${month}${day}_${hour}${minute}.csv"

      // 브라우저 다운로드 방식 사용
      val blob = new dom.Blob(
        js.Array(csvWithBom),
        dom.BlobPropertyBag(`type` = "text/csv;charset=utf-8;")
      )

      val url = dom.URL.createObjectURL(blob)
      val link = dom.document.createElement("a").asInstanceOf[dom.html.Anchor]

      link.href = url
      link.download = fileName
      link.style.display = "none"

      // 강제로 다운로드 속성 설정
      link.setAttribute("download", fileName)
      link.setAttribute("target", "_blank")

      dom.document.body.appendChild(link)

      // 클릭 이벤트 강제 실행
      dom.console.log(s"Triggering download for: $fileName")
      link.click()

      // 약간의 지연 후 정리
      dom.window.setTimeout(
        () => {
          dom.document.body.removeChild(link)
          dom.URL.revokeObjectURL(url)
        },
        1000
      )

      // 다운로드 경로 정보 표시
      val downloadPath = getDownloadPath()
      dom.window.alert(
        s"CSV 파일 다운로드를 시작했습니다!\n\n파일명: $fileName\n\n예상 저장 위치:\n$downloadPath$fileName\n\n※ 브라우저 설정에 따라 다른 폴더에 저장될 수 있습니다."
      )

    } catch {
      case ex: Exception =>
        dom.console.error("CSV export error:", ex)
        dom.window.alert(s"CSV 내보내기 중 오류가 발생했습니다:\n${ex.getMessage}")
    }
  }

  // CSV 필드 이스케이프 처리
  private def escapeCSVField(field: String): String = {
    if (field.contains(",") || field.contains("\"") || field.contains("\n")) {
      // 따옴표가 있으면 두 배로 만들고, 전체를 따옴표로 감싸기
      val escaped = field.replace("\"", "\"\"")
      s"\"$escaped\""
    } else {
      field
    }
  }

  // 브라우저의 기본 다운로드 경로 추정
  private def getDownloadPath(): String = {
    val userAgent = dom.window.navigator.userAgent
    val platform = dom.window.navigator.platform

    // 브라우저 감지
    val browserName =
      if (userAgent.contains("Chrome") && !userAgent.contains("Edg")) {
        "Chrome"
      } else if (userAgent.contains("Firefox")) {
        "Firefox"
      } else if (
        userAgent.contains("Safari") && !userAgent.contains("Chrome")
      ) {
        "Safari"
      } else if (userAgent.contains("Edg")) {
        "Edge"
      } else if (userAgent.contains("Electron")) {
        "Electron"
      } else {
        "브라우저"
      }

    // 운영체제별 기본 다운로드 경로
    val actualPath = if (platform.indexOf("Mac") != -1) {
      // macOS
      "/Users/[사용자명]/Downloads/"
    } else if (platform.indexOf("Win") != -1) {
      // Windows
      "C:\\Users\\[사용자명]\\Downloads\\"
    } else {
      // Linux 등
      "~/Downloads/"
    }

    s"$browserName 기본 다운로드 폴더:\n$actualPath"
  }

  // UI Components
  private def radioButton(
      value: String,
      name: String,
      checkedSignal: Signal[Boolean],
      onSelect: Observer[Unit]
  ): HtmlElement =
    label(
      display := "inline-flex",
      alignItems := "center",
      marginRight := "20px",
      cursor := "pointer",
      input(
        tpe := "radio",
        nameAttr := name,
        checked <-- checkedSignal,
        onChange.mapTo(()) --> onSelect,
        marginRight := "5px"
      ),
      value
    )

  private def scraperControls(): HtmlElement =
    div(
      cls := "scraper-controls",

      // URL Input
      div(
        marginBottom := "15px",
        label(
          display := "block",
          marginBottom := "5px",
          fontWeight := "bold",
          "Gallery URL:"
        ),
        input(
          tpe := "text",
          cls := "url-input",
          placeholder := "https://gall.dcinside.com/mini/vsoop",
          value <-- urlInput,
          onInput.mapToValue --> urlInput,
          width := "100%",
          maxWidth := "500px"
        )
      ),

      // Scraping mode selection
      child <-- urlInput.signal.map { url =>
        if (isValidDCGalleryUrl(url)) {
          div(
            marginBottom := "15px",

            // Radio buttons for mode selection
            div(
              marginBottom := "10px",
              radioButton(
                "페이지 수로 스크래핑",
                "scrapeMode",
                scrapeMode.signal.map(_.isInstanceOf[PageMode]),
                Observer(_ =>
                  scrapeMode.set(
                    PageMode(pagesInput.now().toIntOption.getOrElse(1))
                  )
                )
              ),
              radioButton(
                "날짜별 스크래핑",
                "scrapeMode",
                scrapeMode.signal.map(_.isInstanceOf[DateRangeMode]),
                Observer(_ =>
                  scrapeMode.set(
                    DateRangeMode(
                      if (startDateInput.now().nonEmpty)
                        Some(startDateInput.now())
                      else None,
                      if (endDateInput.now().nonEmpty) Some(endDateInput.now())
                      else None
                    )
                  )
                )
              )
            ),

            // Mode-specific inputs
            child <-- scrapeMode.signal.map {
              case PageMode(_) =>
                div(
                  label(
                    display := "inline-block",
                    marginRight := "10px",
                    "페이지 수:"
                  ),
                  input(
                    tpe := "number",
                    value <-- pagesInput,
                    onInput.mapToValue --> pagesInput.writer,
                    onBlur.mapToValue --> { value =>
                      // 포커스를 잃을 때만 scrapeMode 업데이트
                      value.toIntOption.foreach(pages =>
                        scrapeMode.set(PageMode(pages))
                      )
                    },
                    width := "100px",
                    padding := "5px",
                    minAttr := "1"
                  )
                )

              case DateRangeMode(_, _) =>
                div(
                  div(
                    marginBottom := "10px",
                    label(
                      display := "inline-block",
                      width := "100px",
                      "시작 날짜:"
                    ),
                    input(
                      tpe := "date",
                      value <-- startDateInput,
                      onInput.mapToValue --> startDateInput.writer,
                      onInput.mapToValue --> { _ =>
                        scrapeMode.update {
                          case DateRangeMode(_, end) =>
                            DateRangeMode(
                              if (startDateInput.now().nonEmpty)
                                Some(startDateInput.now())
                              else None,
                              end
                            )
                          case other => other
                        }
                      },
                      width := "150px",
                      padding := "5px"
                    )
                  ),
                  div(
                    label(
                      display := "inline-block",
                      width := "100px",
                      "종료 날짜:"
                    ),
                    input(
                      tpe := "date",
                      value <-- endDateInput,
                      onInput.mapToValue --> endDateInput.writer,
                      onInput.mapToValue --> { _ =>
                        scrapeMode.update {
                          case DateRangeMode(start, _) =>
                            DateRangeMode(
                              start,
                              if (endDateInput.now().nonEmpty)
                                Some(endDateInput.now())
                              else None
                            )
                          case other => other
                        }
                      },
                      width := "150px",
                      padding := "5px"
                    )
                  )
                )
            }
          )
        } else emptyNode
      },

      // Scrape button
      button(
        tpe := "button",
        cls := "scrape-button",
        child <-- isLoading.signal.map { loading =>
          if (loading) "스크래핑 중..." else "스크래핑 시작"
        },
        disabled <-- Signal.combine(isLoading.signal, urlInput.signal).map {
          case (loading, url) => loading || !isValidDCGalleryUrl(url)
        },
        onClick --> { _ =>
          startScraping()
        }
      )
    )

  private def progressDisplay(): HtmlElement =
    div(
      cls := "progress-display",
      child <-- Signal.combine(isLoading.signal, progressInfo.signal).map {
        case (true, Some(progress)) =>
          div(
            cls := "progress-info",
            padding := "15px",
            backgroundColor := "#f0f0f0",
            borderRadius := "5px",
            marginTop := "20px",
            p(
              marginBottom := "5px",
              fontWeight := "bold",
              progress.message
            ),
            progress.totalPages match {
              case Some(total) =>
                p(
                  marginBottom := "5px",
                  s"페이지: ${progress.currentPage} / $total"
                )
              case None =>
                p(marginBottom := "5px", s"현재 페이지: ${progress.currentPage}")
            },
            p(marginBottom := "5px", s"찾은 게시물: ${progress.totalPosts}개"),
            p(s"고유 사용자: ${progress.uniqueUsers}명")
          )
        case _ => emptyNode
      }
    )

  private def resultsDisplay(): HtmlElement =
    div(
      cls := "results-display",
      child <-- Signal.combine(result.signal, errorMessage.signal).map {
        case (Some(res), _) =>
          div(
            marginTop := "20px",
            h3("스크래핑 결과"),
            div(
              padding := "15px",
              backgroundColor := "#f9f9f9",
              borderRadius := "5px",
              p(marginBottom := "10px", b("갤러리 ID: "), res.galleryId),
              (res.startDate, res.endDate) match {
                case (Some(start), Some(end)) =>
                  p(marginBottom := "10px", b("기간: "), s"$start ~ $end")
                case (Some(start), None) =>
                  p(marginBottom := "10px", b("시작일: "), start)
                case (None, Some(end)) =>
                  p(marginBottom := "10px", b("종료일: "), end)
                case _ =>
                  p(
                    marginBottom := "10px",
                    b("스크래핑한 페이지: "),
                    s"${res.pagesScraped}페이지"
                  )
              },
              p(marginBottom := "10px", b("총 게시물: "), s"${res.totalPosts}개"),
              p(marginBottom := "10px", b("고유 사용자: "), s"${res.uniqueUsers}명"),
              div(
                display := "flex",
                justifyContent := "space-between",
                alignItems := "center",
                marginTop := "20px",
                marginBottom := "10px",
                h4("1 ~ 100위 갤창목록"),
                div(
                  display := "flex",
                  gap := "5px",
                  button(
                    "복사",
                    cls := "copy-button",
                    backgroundColor := "#007bff",
                    color := "white",
                    border := "none",
                    padding := "5px 10px",
                    borderRadius := "3px",
                    cursor := "pointer",
                    fontSize := "12px",
                    onClick --> { _ =>
                      copyTableData(res.userStats.take(100))
                    }
                  ),
                  button(
                    "전체선택",
                    backgroundColor := "#28a745",
                    color := "white",
                    border := "none",
                    padding := "5px 10px",
                    borderRadius := "3px",
                    cursor := "pointer",
                    fontSize := "12px",
                    onClick --> { _ =>
                      selectTableContent()
                    }
                  ),
                  button(
                    "CSV 내보내기",
                    backgroundColor := "#ffc107",
                    color := "#212529",
                    border := "none",
                    padding := "5px 10px",
                    borderRadius := "3px",
                    cursor := "pointer",
                    fontSize := "12px",
                    fontWeight := "bold",
                    onClick --> { _ =>
                      exportToCSV(res.userStats.take(100), res.galleryId)
                    }
                  )
                )
              ),
              div(
                backgroundColor := "white",
                border := "1px solid #ddd",
                borderRadius := "3px",
                maxHeight := "400px",
                overflowY := "auto",
                table(
                  width := "100%",
                  borderCollapse := "collapse",
                  thead(
                    tr(
                      backgroundColor := "#f8f9fa",
                      th(
                        "순위",
                        padding := "8px",
                        textAlign := "center",
                        border := "1px solid #ddd",
                        fontWeight := "bold"
                      ),
                      th(
                        "닉네임",
                        padding := "8px",
                        textAlign := "left",
                        border := "1px solid #ddd",
                        fontWeight := "bold"
                      ),
                      th(
                        "식별코드",
                        padding := "8px",
                        textAlign := "center",
                        border := "1px solid #ddd",
                        fontWeight := "bold"
                      ),
                      th(
                        "IP",
                        padding := "8px",
                        textAlign := "center",
                        border := "1px solid #ddd",
                        fontWeight := "bold"
                      ),
                      th(
                        "게시물 수",
                        padding := "8px",
                        textAlign := "center",
                        border := "1px solid #ddd",
                        fontWeight := "bold"
                      )
                    )
                  ),
                  tbody(
                    res.userStats.take(100).zipWithIndex.map {
                      case (user, index) =>
                        tr(
                          backgroundColor := (if (index % 2 == 0) "#ffffff"
                                              else "#f8f9fa"),
                          td(
                            (index + 1).toString,
                            padding := "8px",
                            textAlign := "center",
                            border := "1px solid #ddd"
                          ),
                          td(
                            user.nickname,
                            padding := "8px",
                            textAlign := "left",
                            border := "1px solid #ddd"
                          ),
                          td(
                            user.uid,
                            padding := "8px",
                            textAlign := "center",
                            border := "1px solid #ddd",
                            fontFamily := "monospace"
                          ),
                          td(
                            user.ip,
                            padding := "8px",
                            textAlign := "center",
                            border := "1px solid #ddd",
                            fontFamily := "monospace"
                          ),
                          td(
                            s"${user.count}개",
                            padding := "8px",
                            textAlign := "center",
                            border := "1px solid #ddd"
                          )
                        )
                    }
                  )
                )
              )
            )
          )

        case (None, Some(error)) =>
          div(
            marginTop := "20px",
            cls := "error-message",
            color := "red",
            padding := "15px",
            backgroundColor := "#fee",
            borderRadius := "5px",
            p(b("오류: "), error)
          )

        case _ => emptyNode
      }
    )

  private def startScraping(): Unit = {
    val url = urlInput.now()
    if (!isValidDCGalleryUrl(url)) return

    // Reset state
    isLoading.set(true)
    errorMessage.set(None)
    result.set(None)
    progressInfo.set(None)

    // Subscribe to scraping events
    val eventStream = streamScrape(url, scrapeMode.now())

    eventStream.foreach {
      case ProgressEvent(info) => progressInfo.set(Some(info))
      case CompleteEvent(res) =>
        result.set(Some(res))
        isLoading.set(false)
        progressInfo.set(None)
      case ErrorEvent(msg) =>
        errorMessage.set(Some(msg))
        isLoading.set(false)
        progressInfo.set(None)
    }(unsafeWindowOwner)
  }

  def appElement(): Element = {
    div(
      cls := "app",
      h1("갤창랭킹 수집기"),
      div(
        cls := "container",
        margin := "0 auto",
        scraperControls(),
        progressDisplay(),
        resultsDisplay()
      )
    )
  }
}
